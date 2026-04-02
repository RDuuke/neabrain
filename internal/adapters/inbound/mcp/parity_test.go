package mcp_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	httpadapter "neabrain/internal/adapters/inbound/http"
	"neabrain/internal/adapters/inbound/mcp"
	"neabrain/internal/app"
	"neabrain/internal/domain"
	ports "neabrain/internal/ports/outbound"
)

func TestMCPAndHTTPCreateParity(t *testing.T) {
	payload := map[string]any{
		"content":   "Hello from parity",
		"project":   "alpha",
		"topic_key": "updates",
		"tags": []string{
			"a",
			"b",
		},
		"source": "test",
		"metadata": map[string]any{
			"origin": "http",
		},
	}

	appInstance := newTestApp(t)
	defer func() {
		_ = appInstance.Close()
	}()
	httpObs := createObservationHTTP(t, appInstance, payload)
	if httpObs.ID == "" || httpObs.CreatedAt.IsZero() || httpObs.UpdatedAt.IsZero() {
		t.Fatalf("expected http observation timestamps and id")
	}
	assertObservationFields(t, httpObs, payload)

	mcpApp := newTestApp(t)
	defer func() {
		_ = mcpApp.Close()
	}()
	mcpObs := createObservationMCP(t, mcpApp, payload)
	if mcpObs.ID == "" || mcpObs.CreatedAt.IsZero() || mcpObs.UpdatedAt.IsZero() {
		t.Fatalf("expected mcp observation timestamps and id")
	}
	assertObservationFields(t, mcpObs, payload)
}

func TestMCPAndHTTPInvalidInputParity(t *testing.T) {
	payload := map[string]any{
		"content": "",
	}

	appInstance := newTestApp(t)
	defer func() {
		_ = appInstance.Close()
	}()
	httpCode := createObservationHTTPError(t, appInstance, payload)
	if httpCode != string(domain.ErrorInvalidInput) {
		t.Fatalf("expected http invalid_input, got %s", httpCode)
	}

	mcpApp := newTestApp(t)
	defer func() {
		_ = mcpApp.Close()
	}()
	mcpCode := createObservationMCPError(t, mcpApp, payload)
	if mcpCode != string(domain.ErrorInvalidInput) {
		t.Fatalf("expected mcp invalid_input, got %s", mcpCode)
	}
}

func TestMCPDeadlineTimeoutMapping(t *testing.T) {
	repo := &blockingObservationRepo{wait: 50 * time.Millisecond}
	service := domain.NewObservationService(repo, nil, nil, nil, nil)
	appInstance := &app.App{ObservationService: service}
	server := mcp.NewServer(appInstance)

	args, err := json.Marshal(map[string]any{"id": "obs-timeout"})
	if err != nil {
		t.Fatalf("marshal args: %v", err)
	}
	params, err := json.Marshal(map[string]any{
		"name":        "observation.read",
		"arguments":   json.RawMessage(args),
		"deadline_ms": int64(5),
	})
	if err != nil {
		t.Fatalf("marshal params: %v", err)
	}

	resp := server.Handle(context.Background(), mcp.Request{JSONRPC: "2.0", ID: "1", Method: "tools/call", Params: params})
	if resp.Error == nil || resp.Error.Data == nil {
		t.Fatalf("expected mcp timeout error")
	}
	if resp.Error.Data.Code != "timeout" {
		t.Fatalf("expected timeout code, got %s", resp.Error.Data.Code)
	}
}

func TestMCPCanceledMapping(t *testing.T) {
	repo := &blockingObservationRepo{wait: 50 * time.Millisecond}
	service := domain.NewObservationService(repo, nil, nil, nil, nil)
	appInstance := &app.App{ObservationService: service}
	server := mcp.NewServer(appInstance)

	args, err := json.Marshal(map[string]any{"id": "obs-canceled"})
	if err != nil {
		t.Fatalf("marshal args: %v", err)
	}
	params, err := json.Marshal(map[string]any{
		"name":      "observation.read",
		"arguments": json.RawMessage(args),
	})
	if err != nil {
		t.Fatalf("marshal params: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	resp := server.Handle(ctx, mcp.Request{JSONRPC: "2.0", ID: "1", Method: "tools/call", Params: params})
	if resp.Error == nil || resp.Error.Data == nil {
		t.Fatalf("expected mcp canceled error")
	}
	if resp.Error.Data.Code != "canceled" {
		t.Fatalf("expected canceled code, got %s", resp.Error.Data.Code)
	}
}

func TestMCPSyncStatusAndExport(t *testing.T) {
	appInstance := newTestApp(t)
	defer func() {
		_ = appInstance.Close()
	}()
	server := mcp.NewServer(appInstance)
	syncDir := t.TempDir()

	created := createObservationMCP(t, appInstance, map[string]any{
		"content": "sync me",
		"project": "alpha",
		"source":  "test",
	})
	if created.ID == "" {
		t.Fatal("expected created observation id")
	}

	exportResult := callMCPTool(t, server, "nbn_sync_export", map[string]any{
		"dir":     syncDir,
		"project": "alpha",
	})
	if exportResult["chunk_id"] == "" {
		t.Fatalf("expected chunk_id in export result, got %#v", exportResult)
	}
	if exportResult["count"] != float64(1) {
		t.Fatalf("expected count 1, got %#v", exportResult["count"])
	}
	if exportResult["sync_dir"] != syncDir {
		t.Fatalf("expected sync_dir %q, got %#v", syncDir, exportResult["sync_dir"])
	}

	statusResult := callMCPTool(t, server, "nbn_sync_status", map[string]any{"dir": syncDir})
	if statusResult["SyncDir"] != syncDir {
		t.Fatalf("expected sync_dir %q, got %#v", syncDir, statusResult["SyncDir"])
	}
	if statusResult["TotalChunks"] != float64(1) {
		t.Fatalf("expected total_chunks 1, got %#v", statusResult["TotalChunks"])
	}
	if statusResult["PendingChunks"] != float64(1) {
		t.Fatalf("expected pending_chunks 1, got %#v", statusResult["PendingChunks"])
	}
}

func TestMCPSyncImport(t *testing.T) {
	sourceApp := newTestApp(t)
	defer func() {
		_ = sourceApp.Close()
	}()
	sourceServer := mcp.NewServer(sourceApp)
	syncDir := t.TempDir()

	createObservationMCP(t, sourceApp, map[string]any{
		"content": "importable",
		"project": "alpha",
		"source":  "test",
	})
	callMCPTool(t, sourceServer, "nbn_sync_export", map[string]any{"dir": syncDir})

	targetApp := newTestApp(t)
	defer func() {
		_ = targetApp.Close()
	}()
	targetServer := mcp.NewServer(targetApp)

	importResult := callMCPTool(t, targetServer, "nbn_sync_import", map[string]any{"dir": syncDir})
	if importResult["chunks_processed"] != float64(1) {
		t.Fatalf("expected chunks_processed 1, got %#v", importResult["chunks_processed"])
	}
	if importResult["created"] != float64(1) {
		t.Fatalf("expected created 1, got %#v", importResult["created"])
	}
	if importResult["skipped"] != float64(0) {
		t.Fatalf("expected skipped 0, got %#v", importResult["skipped"])
	}

	listResult := callMCPToolRaw(t, targetServer, "observation.list", map[string]any{"project": "alpha"})
	data, err := json.Marshal(listResult)
	if err != nil {
		t.Fatalf("marshal list result: %v", err)
	}
	var observations []domain.Observation
	if err := json.Unmarshal(data, &observations); err != nil {
		t.Fatalf("decode observations: %v", err)
	}
	if len(observations) != 1 {
		t.Fatalf("expected 1 imported observation, got %d", len(observations))
	}
	if observations[0].Content != "importable" {
		t.Fatalf("expected imported content, got %q", observations[0].Content)
	}
}

func TestMCPTopicsList(t *testing.T) {
	appInstance := newTestApp(t)
	defer func() {
		_ = appInstance.Close()
	}()
	server := mcp.NewServer(appInstance)

	callMCPTool(t, server, "topic.upsert", map[string]any{
		"topic_key":   "auth",
		"name":        "Authentication",
		"description": "auth topic",
	})
	createObservationMCP(t, appInstance, map[string]any{
		"content":   "auth detail",
		"project":   "alpha",
		"topic_key": "auth",
		"source":    "test",
	})

	result := callMCPToolRaw(t, server, "nbn_topics_list", map[string]any{})
	data, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("marshal topics list: %v", err)
	}
	var summaries []domain.TopicSummary
	if err := json.Unmarshal(data, &summaries); err != nil {
		t.Fatalf("decode topic summaries: %v", err)
	}
	if len(summaries) != 1 {
		t.Fatalf("expected 1 topic summary, got %d", len(summaries))
	}
	if summaries[0].TopicKey != "auth" || summaries[0].Count != 1 {
		t.Fatalf("unexpected topic summary: %#v", summaries[0])
	}
}

func TestMCPObservationListFiltersByDisclosureLevel(t *testing.T) {
	appInstance := newTestApp(t)
	defer func() {
		_ = appInstance.Close()
	}()
	server := mcp.NewServer(appInstance)

	createObservationMCP(t, appInstance, map[string]any{
		"content": "public",
		"project": "alpha",
		"tags":    []string{"shared"},
		"source":  "test",
	})
	createObservationMCP(t, appInstance, map[string]any{
		"content": "private",
		"project": "alpha",
		"tags":    []string{"private"},
		"source":  "test",
	})
	createObservationMCP(t, appInstance, map[string]any{
		"content": "sensitive",
		"project": "alpha",
		"tags":    []string{"sensitive"},
		"source":  "test",
	})

	result := callMCPToolRaw(t, server, "observation.list", map[string]any{
		"project":          "alpha",
		"disclosure_level": "low",
	})
	data, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("marshal observations: %v", err)
	}
	var observations []domain.Observation
	if err := json.Unmarshal(data, &observations); err != nil {
		t.Fatalf("decode observations: %v", err)
	}
	if len(observations) != 1 || observations[0].Content != "public" {
		t.Fatalf("unexpected low disclosure observations: %#v", observations)
	}
}

func TestMCPSearchFiltersBySessionDisclosure(t *testing.T) {
	appInstance := newTestApp(t)
	defer func() {
		_ = appInstance.Close()
	}()
	server := mcp.NewServer(appInstance)

	createObservationMCP(t, appInstance, map[string]any{
		"content": "timeout public",
		"project": "alpha",
		"tags":    []string{"shared"},
		"source":  "test",
	})
	createObservationMCP(t, appInstance, map[string]any{
		"content": "timeout private",
		"project": "alpha",
		"tags":    []string{"private"},
		"source":  "test",
	})

	sessionResult := callMCPTool(t, server, "session.open", map[string]any{"disclosure_level": "medium"})
	sessionID, _ := sessionResult["ID"].(string)
	if sessionID == "" {
		t.Fatalf("expected session id, got %#v", sessionResult)
	}

	result := callMCPToolRaw(t, server, "search", map[string]any{
		"query":      "timeout",
		"project":    "alpha",
		"session_id": sessionID,
	})
	data, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("marshal search results: %v", err)
	}
	var observations []domain.Observation
	if err := json.Unmarshal(data, &observations); err != nil {
		t.Fatalf("decode search results: %v", err)
	}
	if len(observations) != 1 || observations[0].Content != "timeout public" {
		t.Fatalf("unexpected session-filtered search results: %#v", observations)
	}
}

func TestMCPTimeline(t *testing.T) {
	appInstance := newTestApp(t)
	defer func() {
		_ = appInstance.Close()
	}()
	server := mcp.NewServer(appInstance)

	first := createObservationMCP(t, appInstance, map[string]any{
		"content": "first",
		"project": "alpha",
		"source":  "test",
	})
	second := createObservationMCP(t, appInstance, map[string]any{
		"content": "second",
		"project": "alpha",
		"source":  "test",
	})
	third := createObservationMCP(t, appInstance, map[string]any{
		"content": "third",
		"project": "alpha",
		"source":  "test",
	})

	result := callMCPToolRaw(t, server, "observation.timeline", map[string]any{
		"id":     second.ID,
		"before": 1,
		"after":  1,
	})
	data, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("marshal timeline: %v", err)
	}
	var timeline domain.TimelineResult
	if err := json.Unmarshal(data, &timeline); err != nil {
		t.Fatalf("decode timeline: %v", err)
	}
	if timeline.Target.ID != second.ID {
		t.Fatalf("expected target %s, got %#v", second.ID, timeline.Target)
	}
	if len(timeline.Before) != 1 || timeline.Before[0].ID != first.ID {
		t.Fatalf("unexpected before observations: %#v", timeline.Before)
	}
	if len(timeline.After) != 1 || timeline.After[0].ID != third.ID {
		t.Fatalf("unexpected after observations: %#v", timeline.After)
	}
}

func newTestApp(t *testing.T) *app.App {
	t.Helper()
	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "neabrain.db")
	configPath := filepath.Join(t.TempDir(), "config.json")
	overrides := ports.ConfigOverrides{
		StoragePath: strPtr(dbPath),
		FTSPath:     strPtr(dbPath),
		ConfigFile:  strPtr(configPath),
	}
	appInstance, err := app.Bootstrap(ctx, overrides)
	if err != nil {
		t.Fatalf("bootstrap: %v", err)
	}
	return appInstance
}

func createObservationHTTP(t *testing.T, appInstance *app.App, payload map[string]any) domain.Observation {
	t.Helper()
	handler := httpadapter.NewHandler(appInstance)
	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	req := httptest.NewRequest(http.MethodPost, "/observations", bytes.NewReader(body))
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("expected status 201, got %d", w.Code)
	}
	var obs domain.Observation
	if err := json.Unmarshal(w.Body.Bytes(), &obs); err != nil {
		t.Fatalf("decode: %v", err)
	}
	return obs
}

func createObservationHTTPError(t *testing.T, appInstance *app.App, payload map[string]any) string {
	t.Helper()
	handler := httpadapter.NewHandler(appInstance)
	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	req := httptest.NewRequest(http.MethodPost, "/observations", bytes.NewReader(body))
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if w.Code == http.StatusCreated {
		t.Fatalf("expected error status, got %d", w.Code)
	}
	var resp struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	return resp.Error.Code
}

func createObservationMCP(t *testing.T, appInstance *app.App, payload map[string]any) domain.Observation {
	t.Helper()
	server := mcp.NewServer(appInstance)
	args, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal args: %v", err)
	}
	params, err := json.Marshal(map[string]any{"name": "observation.create", "arguments": json.RawMessage(args)})
	if err != nil {
		t.Fatalf("marshal params: %v", err)
	}
	resp := server.Handle(context.Background(), mcp.Request{JSONRPC: "2.0", ID: "1", Method: "tools/call", Params: params})
	if resp.Error != nil {
		t.Fatalf("mcp error: %s", resp.Error.Message)
	}
	var obs domain.Observation
	data, err := json.Marshal(resp.Result)
	if err != nil {
		t.Fatalf("marshal result: %v", err)
	}
	if err := json.Unmarshal(data, &obs); err != nil {
		t.Fatalf("decode: %v", err)
	}
	return obs
}

func createObservationMCPError(t *testing.T, appInstance *app.App, payload map[string]any) string {
	t.Helper()
	server := mcp.NewServer(appInstance)
	args, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal args: %v", err)
	}
	params, err := json.Marshal(map[string]any{"name": "observation.create", "arguments": json.RawMessage(args)})
	if err != nil {
		t.Fatalf("marshal params: %v", err)
	}
	resp := server.Handle(context.Background(), mcp.Request{JSONRPC: "2.0", ID: "1", Method: "tools/call", Params: params})
	if resp.Error == nil || resp.Error.Data == nil {
		t.Fatalf("expected mcp error")
	}
	return resp.Error.Data.Code
}

func callMCPTool(t *testing.T, server *mcp.Server, name string, payload map[string]any) map[string]any {
	t.Helper()
	raw := callMCPToolRaw(t, server, name, payload)
	data, err := json.Marshal(raw)
	if err != nil {
		t.Fatalf("marshal result: %v", err)
	}
	var result map[string]any
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("decode result: %v", err)
	}
	return result
}

func callMCPToolRaw(t *testing.T, server *mcp.Server, name string, payload map[string]any) any {
	t.Helper()
	args, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal args: %v", err)
	}
	params, err := json.Marshal(map[string]any{"name": name, "arguments": json.RawMessage(args)})
	if err != nil {
		t.Fatalf("marshal params: %v", err)
	}
	resp := server.Handle(context.Background(), mcp.Request{JSONRPC: "2.0", ID: "1", Method: "tools/call", Params: params})
	if resp.Error != nil {
		t.Fatalf("mcp error calling %s: %s", name, resp.Error.Message)
	}
	return resp.Result
}

func assertObservationFields(t *testing.T, obs domain.Observation, payload map[string]any) {
	t.Helper()
	content := asString(payload["content"])
	project := asString(payload["project"])
	topicKey := asString(payload["topic_key"])
	source := asString(payload["source"])
	if obs.Content != content {
		t.Fatalf("expected content %s, got %s", content, obs.Content)
	}
	if obs.Project != project {
		t.Fatalf("expected project %s, got %s", project, obs.Project)
	}
	if obs.TopicKey != topicKey {
		t.Fatalf("expected topic_key %s, got %s", topicKey, obs.TopicKey)
	}
	if obs.Source != source {
		t.Fatalf("expected source %s, got %s", source, obs.Source)
	}
	tags, _ := payload["tags"].([]string)
	if len(obs.Tags) != len(tags) {
		t.Fatalf("expected tags length %d, got %d", len(tags), len(obs.Tags))
	}
	for i, tag := range tags {
		if obs.Tags[i] != tag {
			t.Fatalf("expected tag %s, got %s", tag, obs.Tags[i])
		}
	}
	metadata, _ := payload["metadata"].(map[string]any)
	if len(obs.Metadata) != len(metadata) {
		t.Fatalf("expected metadata length %d, got %d", len(metadata), len(obs.Metadata))
	}
	for key, value := range metadata {
		if obs.Metadata[key] != value {
			t.Fatalf("expected metadata %s=%v, got %v", key, value, obs.Metadata[key])
		}
	}
}

func asString(value any) string {
	if value == nil {
		return ""
	}
	parsed, ok := value.(string)
	if ok {
		return parsed
	}
	return ""
}

func strPtr(value string) *string {
	return &value
}

type blockingObservationRepo struct {
	wait time.Duration
}

func (r *blockingObservationRepo) Create(ctx context.Context, observation domain.Observation) (domain.Observation, error) {
	return domain.Observation{}, errors.New("unexpected Create call")
}

func (r *blockingObservationRepo) GetByID(ctx context.Context, id string, includeDeleted bool) (domain.Observation, error) {
	select {
	case <-ctx.Done():
		return domain.Observation{}, ctx.Err()
	case <-time.After(r.wait):
		return domain.Observation{}, errors.New("deadline not enforced")
	}
}

func (r *blockingObservationRepo) Update(ctx context.Context, observation domain.Observation) (domain.Observation, error) {
	return domain.Observation{}, errors.New("unexpected Update call")
}

func (r *blockingObservationRepo) List(ctx context.Context, filter domain.ObservationListFilter) ([]domain.Observation, error) {
	return nil, errors.New("unexpected List call")
}

func (r *blockingObservationRepo) FindAround(ctx context.Context, id string, before, after int, includeDeleted bool) (domain.TimelineResult, error) {
	return domain.TimelineResult{}, errors.New("unexpected FindAround call")
}

func (r *blockingObservationRepo) SoftDelete(ctx context.Context, id string, deletedAt time.Time) (domain.Observation, error) {
	return domain.Observation{}, errors.New("unexpected SoftDelete call")
}

func (r *blockingObservationRepo) FindByContent(ctx context.Context, content string, project string, includeDeleted bool) ([]domain.Observation, error) {
	return nil, errors.New("unexpected FindByContent call")
}

func (r *blockingObservationRepo) ListProjects(ctx context.Context) ([]domain.ProjectSummary, error) {
	return nil, errors.New("unexpected ListProjects call")
}

func (r *blockingObservationRepo) RenameProject(ctx context.Context, oldName, newName string) (int, error) {
	return 0, errors.New("unexpected RenameProject call")
}

func (r *blockingObservationRepo) GetStats(ctx context.Context) (domain.ObservationStats, error) {
	return domain.ObservationStats{}, errors.New("unexpected GetStats call")
}
