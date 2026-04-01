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

func (r *blockingObservationRepo) SoftDelete(ctx context.Context, id string, deletedAt time.Time) (domain.Observation, error) {
	return domain.Observation{}, errors.New("unexpected SoftDelete call")
}

func (r *blockingObservationRepo) FindByContent(ctx context.Context, content string, project string, includeDeleted bool) ([]domain.Observation, error) {
	return nil, errors.New("unexpected FindByContent call")
}
