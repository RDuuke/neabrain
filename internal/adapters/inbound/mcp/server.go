package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"neabrain/internal/app"
	"neabrain/internal/domain"
)

// Profile controls which tools are exposed by the MCP server.
// "agent" — read/write tools useful for AI agents (default).
// "admin" — agent tools plus destructive/administrative tools.
// "all"   — all registered tools (alias for admin).
type Profile string

const (
	ProfileAgent Profile = "agent"
	ProfileAdmin Profile = "admin"
	ProfileAll   Profile = "all"
)

// Server handles MCP-style tool requests over JSON-RPC.
type Server struct {
	app     *app.App
	profile Profile
}

// NewServer constructs a MCP server with the default (agent) profile.
func NewServer(appInstance *app.App) *Server {
	return &Server{app: appInstance, profile: ProfileAgent}
}

// NewServerWithProfile constructs a MCP server with an explicit profile.
func NewServerWithProfile(appInstance *app.App, profile Profile) *Server {
	if profile == "" {
		profile = ProfileAgent
	}
	return &Server{app: appInstance, profile: profile}
}

// Serve processes JSON-RPC requests from in and writes responses to out.
func (s *Server) Serve(ctx context.Context, in io.Reader, out io.Writer) error {
	if s == nil || s.app == nil {
		return fmt.Errorf("mcp server requires app")
	}
	if in == nil || out == nil {
		return fmt.Errorf("mcp server requires io")
	}

	decoder := json.NewDecoder(in)
	encoder := json.NewEncoder(out)
	encoder.SetEscapeHTML(false)

	for {
		var req Request
		if err := decoder.Decode(&req); err != nil {
			if errors.Is(err, io.EOF) {
				return nil
			}
			return err
		}
		resp := s.Handle(ctx, req)
		if err := encoder.Encode(resp); err != nil {
			return err
		}
	}
}

// Handle processes a single JSON-RPC request.
func (s *Server) Handle(ctx context.Context, req Request) Response {
	if req.JSONRPC == "" {
		req.JSONRPC = "2.0"
	}
	if s != nil && s.app != nil {
		if s.app.Logger != nil {
			s.app.Logger.Info("mcp request", map[string]any{"method": req.Method})
		}
		if s.app.Metrics != nil {
			s.app.Metrics.Inc("adapter.mcp.request")
		}
	}

	switch req.Method {
	case "initialize":
		return Response{JSONRPC: "2.0", ID: req.ID, Result: map[string]any{
			"protocolVersion": "2024-11-05",
			"capabilities": map[string]any{
				"tools": map[string]any{},
			},
			"serverInfo": map[string]any{
				"name":    "neabrain",
				"version": "0.1.0",
			},
		}}
	case "initialized", "notifications/initialized":
		return Response{JSONRPC: "2.0", ID: req.ID, Result: map[string]any{}}
	case "tools/list":
		return Response{JSONRPC: "2.0", ID: req.ID, Result: toolsListResult{Tools: s.filteredTools()}}
	case "tools/call":
		return s.handleToolsCall(ctx, req)
	default:
		return Response{JSONRPC: "2.0", ID: req.ID, Error: &RPCError{Code: -32601, Message: "method not found"}}
	}
}

type Request struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      any             `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params"`
}

type Response struct {
	JSONRPC string    `json:"jsonrpc"`
	ID      any       `json:"id"`
	Result  any       `json:"result,omitempty"`
	Error   *RPCError `json:"error,omitempty"`
}

type RPCError struct {
	Code    int        `json:"code"`
	Message string     `json:"message"`
	Data    *ErrorData `json:"data,omitempty"`
}

type ErrorData struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type toolsCallParams struct {
	Name       string          `json:"name"`
	Arguments  json.RawMessage `json:"arguments"`
	DeadlineMS *int64          `json:"deadline_ms"`
}

type toolsListResult struct {
	Tools []ToolDefinition `json:"tools"`
}

// ToolAnnotations provides MCP 2024-11-05 hints to clients.
type ToolAnnotations struct {
	ReadOnlyHint    bool `json:"readOnlyHint,omitempty"`
	DestructiveHint bool `json:"destructiveHint,omitempty"`
	IdempotentHint  bool `json:"idempotentHint,omitempty"`
}

// ToolDefinition describes an MCP tool including schema and behavioral hints.
type ToolDefinition struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	InputSchema map[string]any  `json:"inputSchema"`
	Annotations ToolAnnotations `json:"annotations,omitempty"`
	// adminOnly marks tools hidden from the agent profile.
	adminOnly bool `json:"-"`
}

type observationCreateArgs struct {
	Content        string         `json:"content"`
	Project        string         `json:"project"`
	TopicKey       string         `json:"topic_key"`
	Tags           []string       `json:"tags"`
	Source         string         `json:"source"`
	Metadata       map[string]any `json:"metadata"`
	AllowDuplicate bool           `json:"allow_duplicate"`
}

type observationReadArgs struct {
	ID             string `json:"id"`
	IncludeDeleted bool   `json:"include_deleted"`
}

type observationUpdateArgs struct {
	ID       string          `json:"id"`
	Content  *string         `json:"content"`
	Project  *string         `json:"project"`
	TopicKey *string         `json:"topic_key"`
	Tags     *[]string       `json:"tags"`
	Source   *string         `json:"source"`
	Metadata *map[string]any `json:"metadata"`
}

type observationListArgs struct {
	Project        string   `json:"project"`
	TopicKey       string   `json:"topic_key"`
	Tags           []string `json:"tags"`
	IncludeDeleted bool     `json:"include_deleted"`
}

type observationDeleteArgs struct {
	ID string `json:"id"`
}

type searchArgs struct {
	Query          string   `json:"query"`
	Project        string   `json:"project"`
	TopicKey       string   `json:"topic_key"`
	Tags           []string `json:"tags"`
	IncludeDeleted bool     `json:"include_deleted"`
}

type topicUpsertArgs struct {
	TopicKey    string         `json:"topic_key"`
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Metadata    map[string]any `json:"metadata"`
}

type sessionOpenArgs struct {
	DisclosureLevel string `json:"disclosure_level"`
}

type sessionResumeArgs struct {
	ID string `json:"id"`
}

type sessionUpdateArgs struct {
	ID              string `json:"id"`
	DisclosureLevel string `json:"disclosure_level"`
}

type nbnSessionSummaryArgs struct {
	Summary  string         `json:"summary"`
	Project  string         `json:"project"`
	TopicKey string         `json:"topic_key"`
	Tags     []string       `json:"tags"`
	Metadata map[string]any `json:"metadata"`
}

type nbnContextArgs struct {
	Query          string   `json:"query"`
	Project        string   `json:"project"`
	TopicKey       string   `json:"topic_key"`
	Tags           []string `json:"tags"`
	IncludeDeleted bool     `json:"include_deleted"`
}

type nbnExportArgs struct {
	Project        string   `json:"project"`
	TopicKey       string   `json:"topic_key"`
	Tags           []string `json:"tags"`
	IncludeDeleted bool     `json:"include_deleted"`
}

type nbnProjectsRenameArgs struct {
	From string `json:"from"`
	To   string `json:"to"`
}

type nbnMergeProjectsArgs struct {
	From []string `json:"from"`
	To   string   `json:"to"`
}

type nbnCapturePassiveArgs struct {
	Text     string   `json:"text"`
	Project  string   `json:"project"`
	TopicKey string   `json:"topic_key"`
	Tags     []string `json:"tags"`
}

func (s *Server) handleToolsCall(ctx context.Context, req Request) Response {
	var params toolsCallParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return Response{JSONRPC: "2.0", ID: req.ID, Error: &RPCError{Code: -32602, Message: "invalid params"}}
	}
	name := strings.TrimSpace(params.Name)
	if name == "" {
		return Response{JSONRPC: "2.0", ID: req.ID, Error: &RPCError{Code: -32602, Message: "tool name required"}}
	}
	callCtx := ctx
	if params.DeadlineMS != nil && *params.DeadlineMS > 0 {
		var cancel context.CancelFunc
		callCtx, cancel = context.WithTimeout(ctx, time.Duration(*params.DeadlineMS)*time.Millisecond)
		defer cancel()
	}
	if s != nil && s.app != nil {
		if s.app.Logger != nil {
			s.app.Logger.Info("mcp tool call", map[string]any{"tool": name})
		}
		if s.app.Metrics != nil {
			s.app.Metrics.Inc("adapter.mcp.tool." + name)
		}
	}

	switch name {
	case "nbn_session_summary":
		var args nbnSessionSummaryArgs
		if err := json.Unmarshal(params.Arguments, &args); err != nil {
			return Response{JSONRPC: "2.0", ID: req.ID, Error: &RPCError{Code: -32602, Message: "invalid nbn_session_summary args"}}
		}
		tags := args.Tags
		if len(tags) == 0 {
			tags = []string{"opencode", "session_summary"}
		}
		created, err := s.app.ObservationService.Create(callCtx, domain.ObservationCreateInput{
			Content:        args.Summary,
			Project:        pickProject(args.Project, s.app.Config.DefaultProject),
			TopicKey:       args.TopicKey,
			Tags:           tags,
			Source:         "opencode",
			Metadata:       args.Metadata,
			AllowDuplicate: true,
		})
		if err != nil {
			return Response{JSONRPC: "2.0", ID: req.ID, Error: rpcErrorFrom(err)}
		}
		return Response{JSONRPC: "2.0", ID: req.ID, Result: created}
	case "nbn_context":
		var args nbnContextArgs
		if err := json.Unmarshal(params.Arguments, &args); err != nil {
			return Response{JSONRPC: "2.0", ID: req.ID, Error: &RPCError{Code: -32602, Message: "invalid nbn_context args"}}
		}
		results, err := s.app.SearchService.Search(callCtx, args.Query, domain.SearchFilter{
			Project:        pickProject(args.Project, s.app.Config.DefaultProject),
			TopicKey:       args.TopicKey,
			Tags:           args.Tags,
			IncludeDeleted: args.IncludeDeleted,
		})
		if err != nil {
			return Response{JSONRPC: "2.0", ID: req.ID, Error: rpcErrorFrom(err)}
		}
		return Response{JSONRPC: "2.0", ID: req.ID, Result: results}
	}

	switch name {
	case "nbn_stats":
		stats, err := s.app.ObservationService.GetStats(callCtx)
		if err != nil {
			return Response{JSONRPC: "2.0", ID: req.ID, Error: rpcErrorFrom(err)}
		}
		return Response{JSONRPC: "2.0", ID: req.ID, Result: stats}
	case "nbn_capture_passive":
		var args nbnCapturePassiveArgs
		if err := json.Unmarshal(params.Arguments, &args); err != nil {
			return Response{JSONRPC: "2.0", ID: req.ID, Error: &RPCError{Code: -32602, Message: "invalid nbn_capture_passive args"}}
		}
		tags := args.Tags
		if len(tags) == 0 {
			tags = []string{"passive"}
		}
		created, err := s.app.ObservationService.Create(callCtx, domain.ObservationCreateInput{
			Content:        strings.TrimSpace(args.Text),
			Project:        pickProject(args.Project, s.app.Config.DefaultProject),
			TopicKey:       args.TopicKey,
			Tags:           tags,
			Source:         "passive",
			AllowDuplicate: false,
		})
		if err != nil {
			return Response{JSONRPC: "2.0", ID: req.ID, Error: rpcErrorFrom(err)}
		}
		return Response{JSONRPC: "2.0", ID: req.ID, Result: created}
	case "nbn_merge_projects":
		var args nbnMergeProjectsArgs
		if err := json.Unmarshal(params.Arguments, &args); err != nil {
			return Response{JSONRPC: "2.0", ID: req.ID, Error: &RPCError{Code: -32602, Message: "invalid nbn_merge_projects args"}}
		}
		count, err := s.app.ObservationService.MergeProjects(callCtx, args.From, args.To)
		if err != nil {
			return Response{JSONRPC: "2.0", ID: req.ID, Error: rpcErrorFrom(err)}
		}
		return Response{JSONRPC: "2.0", ID: req.ID, Result: map[string]any{"merged": count}}
	case "nbn_export":
		var args nbnExportArgs
		if err := json.Unmarshal(params.Arguments, &args); err != nil {
			return Response{JSONRPC: "2.0", ID: req.ID, Error: &RPCError{Code: -32602, Message: "invalid nbn_export args"}}
		}
		observations, err := s.app.ObservationService.List(callCtx, domain.ObservationListFilter{
			Project:        pickProject(args.Project, s.app.Config.DefaultProject),
			TopicKey:       args.TopicKey,
			Tags:           args.Tags,
			IncludeDeleted: args.IncludeDeleted,
		})
		if err != nil {
			return Response{JSONRPC: "2.0", ID: req.ID, Error: rpcErrorFrom(err)}
		}
		return Response{JSONRPC: "2.0", ID: req.ID, Result: observations}
	case "nbn_projects_list":
		summaries, err := s.app.ObservationService.ListProjects(callCtx)
		if err != nil {
			return Response{JSONRPC: "2.0", ID: req.ID, Error: rpcErrorFrom(err)}
		}
		return Response{JSONRPC: "2.0", ID: req.ID, Result: summaries}
	case "nbn_projects_rename":
		var args nbnProjectsRenameArgs
		if err := json.Unmarshal(params.Arguments, &args); err != nil {
			return Response{JSONRPC: "2.0", ID: req.ID, Error: &RPCError{Code: -32602, Message: "invalid nbn_projects_rename args"}}
		}
		count, err := s.app.ObservationService.RenameProject(callCtx, args.From, args.To)
		if err != nil {
			return Response{JSONRPC: "2.0", ID: req.ID, Error: rpcErrorFrom(err)}
		}
		return Response{JSONRPC: "2.0", ID: req.ID, Result: map[string]any{"renamed": count}}
	}

	alias := map[string]string{
		"nbn_observation_create":        "observation.create",
		"nbn_observation_read":          "observation.read",
		"nbn_observation_update":        "observation.update",
		"nbn_observation_list":          "observation.list",
		"nbn_observation_delete":        "observation.delete",
		"nbn_search":                    "search",
		"nbn_topic_upsert":              "topic.upsert",
		"nbn_session_open":              "session.open",
		"nbn_session_resume":            "session.resume",
		"nbn_session_update_disclosure": "session.update_disclosure",
		"nbn_config_show":               "config.show",
	}
	if mapped, ok := alias[name]; ok {
		name = mapped
	}

	switch name {
	case "observation.create":
		var args observationCreateArgs
		if err := json.Unmarshal(params.Arguments, &args); err != nil {
			return Response{JSONRPC: "2.0", ID: req.ID, Error: &RPCError{Code: -32602, Message: "invalid observation.create args"}}
		}
		created, err := s.app.ObservationService.Create(callCtx, domain.ObservationCreateInput{
			Content:        args.Content,
			Project:        pickProject(args.Project, s.app.Config.DefaultProject),
			TopicKey:       args.TopicKey,
			Tags:           args.Tags,
			Source:         args.Source,
			Metadata:       args.Metadata,
			AllowDuplicate: args.AllowDuplicate,
		})
		if err != nil {
			return Response{JSONRPC: "2.0", ID: req.ID, Error: rpcErrorFrom(err)}
		}
		return Response{JSONRPC: "2.0", ID: req.ID, Result: created}
	case "observation.read":
		var args observationReadArgs
		if err := json.Unmarshal(params.Arguments, &args); err != nil {
			return Response{JSONRPC: "2.0", ID: req.ID, Error: &RPCError{Code: -32602, Message: "invalid observation.read args"}}
		}
		observation, err := s.app.ObservationService.Read(callCtx, args.ID, args.IncludeDeleted)
		if err != nil {
			return Response{JSONRPC: "2.0", ID: req.ID, Error: rpcErrorFrom(err)}
		}
		return Response{JSONRPC: "2.0", ID: req.ID, Result: observation}
	case "observation.update":
		var args observationUpdateArgs
		if err := json.Unmarshal(params.Arguments, &args); err != nil {
			return Response{JSONRPC: "2.0", ID: req.ID, Error: &RPCError{Code: -32602, Message: "invalid observation.update args"}}
		}
		input := domain.ObservationUpdateInput{ID: args.ID}
		if args.Content != nil {
			input.Content = args.Content
		}
		if args.Project != nil {
			input.Project = args.Project
		}
		if args.TopicKey != nil {
			input.TopicKey = args.TopicKey
		}
		if args.Tags != nil {
			input.Tags = *args.Tags
		}
		if args.Source != nil {
			input.Source = args.Source
		}
		if args.Metadata != nil {
			input.Metadata = *args.Metadata
		}
		updated, err := s.app.ObservationService.Update(callCtx, input)
		if err != nil {
			return Response{JSONRPC: "2.0", ID: req.ID, Error: rpcErrorFrom(err)}
		}
		return Response{JSONRPC: "2.0", ID: req.ID, Result: updated}
	case "observation.list":
		var args observationListArgs
		if err := json.Unmarshal(params.Arguments, &args); err != nil {
			return Response{JSONRPC: "2.0", ID: req.ID, Error: &RPCError{Code: -32602, Message: "invalid observation.list args"}}
		}
		observations, err := s.app.ObservationService.List(callCtx, domain.ObservationListFilter{
			Project:        pickProject(args.Project, s.app.Config.DefaultProject),
			TopicKey:       args.TopicKey,
			Tags:           args.Tags,
			IncludeDeleted: args.IncludeDeleted,
		})
		if err != nil {
			return Response{JSONRPC: "2.0", ID: req.ID, Error: rpcErrorFrom(err)}
		}
		return Response{JSONRPC: "2.0", ID: req.ID, Result: observations}
	case "observation.delete":
		var args observationDeleteArgs
		if err := json.Unmarshal(params.Arguments, &args); err != nil {
			return Response{JSONRPC: "2.0", ID: req.ID, Error: &RPCError{Code: -32602, Message: "invalid observation.delete args"}}
		}
		deleted, err := s.app.ObservationService.SoftDelete(callCtx, args.ID)
		if err != nil {
			return Response{JSONRPC: "2.0", ID: req.ID, Error: rpcErrorFrom(err)}
		}
		return Response{JSONRPC: "2.0", ID: req.ID, Result: deleted}
	case "search":
		var args searchArgs
		if err := json.Unmarshal(params.Arguments, &args); err != nil {
			return Response{JSONRPC: "2.0", ID: req.ID, Error: &RPCError{Code: -32602, Message: "invalid search args"}}
		}
		results, err := s.app.SearchService.Search(callCtx, args.Query, domain.SearchFilter{
			Project:        pickProject(args.Project, s.app.Config.DefaultProject),
			TopicKey:       args.TopicKey,
			Tags:           args.Tags,
			IncludeDeleted: args.IncludeDeleted,
		})
		if err != nil {
			return Response{JSONRPC: "2.0", ID: req.ID, Error: rpcErrorFrom(err)}
		}
		return Response{JSONRPC: "2.0", ID: req.ID, Result: results}
	case "topic.upsert":
		var args topicUpsertArgs
		if err := json.Unmarshal(params.Arguments, &args); err != nil {
			return Response{JSONRPC: "2.0", ID: req.ID, Error: &RPCError{Code: -32602, Message: "invalid topic.upsert args"}}
		}
		topic, err := s.app.TopicService.UpsertByTopicKey(callCtx, domain.TopicUpsertInput{
			TopicKey:    args.TopicKey,
			Name:        args.Name,
			Description: args.Description,
			Metadata:    args.Metadata,
		})
		if err != nil {
			return Response{JSONRPC: "2.0", ID: req.ID, Error: rpcErrorFrom(err)}
		}
		return Response{JSONRPC: "2.0", ID: req.ID, Result: topic}
	case "session.open":
		var args sessionOpenArgs
		if err := json.Unmarshal(params.Arguments, &args); err != nil {
			return Response{JSONRPC: "2.0", ID: req.ID, Error: &RPCError{Code: -32602, Message: "invalid session.open args"}}
		}
		session, err := s.app.SessionService.Open(callCtx, domain.SessionOpenInput{DisclosureLevel: args.DisclosureLevel})
		if err != nil {
			return Response{JSONRPC: "2.0", ID: req.ID, Error: rpcErrorFrom(err)}
		}
		return Response{JSONRPC: "2.0", ID: req.ID, Result: session}
	case "session.resume":
		var args sessionResumeArgs
		if err := json.Unmarshal(params.Arguments, &args); err != nil {
			return Response{JSONRPC: "2.0", ID: req.ID, Error: &RPCError{Code: -32602, Message: "invalid session.resume args"}}
		}
		session, err := s.app.SessionService.Resume(callCtx, args.ID)
		if err != nil {
			return Response{JSONRPC: "2.0", ID: req.ID, Error: rpcErrorFrom(err)}
		}
		return Response{JSONRPC: "2.0", ID: req.ID, Result: session}
	case "session.update_disclosure":
		var args sessionUpdateArgs
		if err := json.Unmarshal(params.Arguments, &args); err != nil {
			return Response{JSONRPC: "2.0", ID: req.ID, Error: &RPCError{Code: -32602, Message: "invalid session.update_disclosure args"}}
		}
		session, err := s.app.SessionService.UpdateDisclosure(callCtx, args.ID, args.DisclosureLevel)
		if err != nil {
			return Response{JSONRPC: "2.0", ID: req.ID, Error: rpcErrorFrom(err)}
		}
		return Response{JSONRPC: "2.0", ID: req.ID, Result: session}
	case "config.show":
		return Response{JSONRPC: "2.0", ID: req.ID, Result: s.app.Config}
	default:
		return Response{JSONRPC: "2.0", ID: req.ID, Error: &RPCError{Code: -32601, Message: "unknown tool"}}
	}
}

// filteredTools returns the tool list appropriate for the server profile.
func (s *Server) filteredTools() []ToolDefinition {
	all := toolDefinitions()
	if s.profile == ProfileAdmin || s.profile == ProfileAll {
		return all
	}
	out := make([]ToolDefinition, 0, len(all))
	for _, t := range all {
		if !t.adminOnly {
			out = append(out, t)
		}
	}
	return out
}

func toolDefinitions() []ToolDefinition {
	ro := ToolAnnotations{ReadOnlyHint: true}
	write := ToolAnnotations{}
	idempotent := ToolAnnotations{IdempotentHint: true}
	destructive := ToolAnnotations{DestructiveHint: true}

	obsCreateSchema := schemaObject(map[string]any{
		"content":         schemaString(),
		"project":         schemaString(),
		"topic_key":       schemaString(),
		"tags":            schemaStringArray(),
		"source":          schemaString(),
		"metadata":        schemaObjectAny(),
		"allow_duplicate": schemaBool(),
	}, "content")
	obsReadSchema := schemaObject(map[string]any{
		"id":              schemaString(),
		"include_deleted": schemaBool(),
	}, "id")
	obsUpdateSchema := schemaObject(map[string]any{
		"id":        schemaString(),
		"content":   schemaString(),
		"project":   schemaString(),
		"topic_key": schemaString(),
		"tags":      schemaStringArray(),
		"source":    schemaString(),
		"metadata":  schemaObjectAny(),
	}, "id")
	obsListSchema := schemaObject(map[string]any{
		"project":         schemaString(),
		"topic_key":       schemaString(),
		"tags":            schemaStringArray(),
		"include_deleted": schemaBool(),
	})
	obsDeleteSchema := schemaObject(map[string]any{"id": schemaString()}, "id")
	searchSchema := schemaObject(map[string]any{
		"query":           schemaString(),
		"project":         schemaString(),
		"topic_key":       schemaString(),
		"tags":            schemaStringArray(),
		"include_deleted": schemaBool(),
	}, "query")
	topicUpsertSchema := schemaObject(map[string]any{
		"topic_key":   schemaString(),
		"name":        schemaString(),
		"description": schemaString(),
		"metadata":    schemaObjectAny(),
	}, "topic_key")

	return []ToolDefinition{
		// ── Observations ────────────────────────────────────────────────────────
		{Name: "observation.create", Annotations: write,
			Description: "Persist a memory observation. Include what happened, why it matters, and where in the codebase. Use topic_key to group related observations so they can evolve together over time.",
			InputSchema: obsCreateSchema},
		{Name: "nbn_observation_create", Annotations: write,
			Description: "Alias for observation.create.",
			InputSchema: obsCreateSchema},

		{Name: "observation.read", Annotations: ro,
			Description: "Retrieve full content of a single observation by ID. Use after search when the preview is truncated.",
			InputSchema: obsReadSchema},
		{Name: "nbn_observation_read", Annotations: ro,
			Description: "Alias for observation.read.",
			InputSchema: obsReadSchema},

		{Name: "observation.update", Annotations: idempotent,
			Description: "Update fields of an existing observation. Only supplied fields are changed.",
			InputSchema: obsUpdateSchema},
		{Name: "nbn_observation_update", Annotations: idempotent,
			Description: "Alias for observation.update.",
			InputSchema: obsUpdateSchema},

		{Name: "observation.list", Annotations: ro,
			Description: "List observations with optional filters. Returns full objects; prefer nbn_search for keyword queries.",
			InputSchema: obsListSchema},
		{Name: "nbn_observation_list", Annotations: ro,
			Description: "Alias for observation.list.",
			InputSchema: obsListSchema},

		{Name: "observation.delete", Annotations: ToolAnnotations{DestructiveHint: true},
			Description: "Soft-delete an observation. It remains queryable with include_deleted=true.",
			InputSchema: obsDeleteSchema},
		{Name: "nbn_observation_delete", Annotations: ToolAnnotations{DestructiveHint: true},
			Description: "Alias for observation.delete.",
			InputSchema: obsDeleteSchema},

		// ── Search ──────────────────────────────────────────────────────────────
		{Name: "search", Annotations: ro,
			Description: "Full-text search across all observations. Returns observations ranked by relevance. Filter by project, topic_key, or tags to narrow scope.",
			InputSchema: searchSchema},
		{Name: "nbn_search", Annotations: ro,
			Description: "Alias for search.",
			InputSchema: searchSchema},
		{Name: "nbn_context", Annotations: ro,
			Description: "Retrieve relevant context for the current task. Use before starting work to recall prior decisions and patterns. Same as search but semantically scoped to context retrieval.",
			InputSchema: searchSchema},

		// ── Topics ──────────────────────────────────────────────────────────────
		{Name: "topic.upsert", Annotations: idempotent,
			Description: "Create or update a named topic. Topics group observations by domain (e.g. 'auth', 'database-schema'). Use a stable topic_key across sessions.",
			InputSchema: topicUpsertSchema},
		{Name: "nbn_topic_upsert", Annotations: idempotent,
			Description: "Alias for topic.upsert.",
			InputSchema: topicUpsertSchema},

		// ── Sessions ────────────────────────────────────────────────────────────
		{Name: "session.open", Annotations: write,
			Description: "Open a new session to track a work context. disclosure_level controls what context is shared back (e.g. 'low', 'high').",
			InputSchema: schemaObject(map[string]any{"disclosure_level": schemaString()}, "disclosure_level")},
		{Name: "nbn_session_open", Annotations: write,
			Description: "Alias for session.open.",
			InputSchema: schemaObject(map[string]any{"disclosure_level": schemaString()}, "disclosure_level")},

		{Name: "session.resume", Annotations: ro,
			Description: "Resume a previously opened session by ID.",
			InputSchema: schemaObject(map[string]any{"id": schemaString()}, "id")},
		{Name: "nbn_session_resume", Annotations: ro,
			Description: "Alias for session.resume.",
			InputSchema: schemaObject(map[string]any{"id": schemaString()}, "id")},

		{Name: "session.update_disclosure", Annotations: idempotent,
			Description: "Change the disclosure level of an open session.",
			InputSchema: schemaObject(map[string]any{
				"id":               schemaString(),
				"disclosure_level": schemaString(),
			}, "id", "disclosure_level")},
		{Name: "nbn_session_update_disclosure", Annotations: idempotent,
			Description: "Alias for session.update_disclosure.",
			InputSchema: schemaObject(map[string]any{
				"id":               schemaString(),
				"disclosure_level": schemaString(),
			}, "id", "disclosure_level")},

		// ── Config ──────────────────────────────────────────────────────────────
		{Name: "config.show", Annotations: ro,
			Description: "Show the effective configuration (storage paths, default project, dedupe policy).",
			InputSchema: schemaObject(map[string]any{})},
		{Name: "nbn_config_show", Annotations: ro,
			Description: "Alias for config.show.",
			InputSchema: schemaObject(map[string]any{})},

		// ── OpenCode helpers ─────────────────────────────────────────────────────
		{Name: "nbn_session_summary", Annotations: write,
			Description: "Store a structured end-of-session summary. Call at compaction time. Include: Goal, Key Decisions, Discoveries, Files Changed.",
			InputSchema: schemaObject(map[string]any{
				"summary":   schemaString(),
				"project":   schemaString(),
				"topic_key": schemaString(),
				"tags":      schemaStringArray(),
				"metadata":  schemaObjectAny(),
			}, "summary")},

		// ── Passive capture ──────────────────────────────────────────────────────
		{Name: "nbn_capture_passive", Annotations: write,
			Description: "Extract and store a learning from text without explicit structuring. Use when you notice something worth preserving mid-conversation that doesn't need full observation metadata.",
			InputSchema: schemaObject(map[string]any{
				"text":      schemaString(),
				"project":   schemaString(),
				"topic_key": schemaString(),
				"tags":      schemaStringArray(),
			}, "text")},

		// ── Export & Projects ────────────────────────────────────────────────────
		{Name: "nbn_export", Annotations: ro,
			Description: "Export observations as a JSON array. Use for backup, migration, or sharing memory across environments.",
			InputSchema: schemaObject(map[string]any{
				"project":         schemaString(),
				"topic_key":       schemaString(),
				"tags":            schemaStringArray(),
				"include_deleted": schemaBool(),
			})},

		{Name: "nbn_projects_list", Annotations: ro,
			Description: "List all active projects with their observation counts.",
			InputSchema: schemaObject(map[string]any{})},

		{Name: "nbn_projects_rename", Annotations: ToolAnnotations{IdempotentHint: true, DestructiveHint: true},
			Description: "Rename a single project across all observations.",
			InputSchema: schemaObject(map[string]any{
				"from": schemaString(),
				"to":   schemaString(),
			}, "from", "to")},

		// ── Admin tools (hidden from agent profile) ──────────────────────────────
		{Name: "nbn_stats", Annotations: ro, adminOnly: true,
			Description: "Return aggregate counts: active observations, soft-deleted observations, and distinct projects.",
			InputSchema: schemaObject(map[string]any{})},

		{Name: "nbn_merge_projects", Annotations: destructive, adminOnly: true,
			Description: "Merge multiple project name variants into one target project. All observations in 'from' projects are reassigned to 'to'. Irreversible.",
			InputSchema: schemaObject(map[string]any{
				"from": schemaStringArray(),
				"to":   schemaString(),
			}, "from", "to")},
	}
}

func schemaObject(properties map[string]any, required ...string) map[string]any {
	schema := map[string]any{"type": "object", "properties": properties}
	if len(required) > 0 {
		schema["required"] = required
	}
	return schema
}

func schemaString() map[string]any {
	return map[string]any{"type": "string"}
}

func schemaBool() map[string]any {
	return map[string]any{"type": "boolean"}
}

func schemaStringArray() map[string]any {
	return map[string]any{"type": "array", "items": map[string]any{"type": "string"}}
}

func schemaObjectAny() map[string]any {
	return map[string]any{"type": "object", "additionalProperties": true}
}

func rpcErrorFrom(err error) *RPCError {
	if err == nil {
		return nil
	}
	if errors.Is(err, context.DeadlineExceeded) {
		message := "request deadline exceeded"
		return &RPCError{
			Code:    -32000,
			Message: message,
			Data:    &ErrorData{Code: "timeout", Message: message},
		}
	}
	if errors.Is(err, context.Canceled) {
		message := "request canceled"
		return &RPCError{
			Code:    -32000,
			Message: message,
			Data:    &ErrorData{Code: "canceled", Message: message},
		}
	}
	var domainErr domain.DomainError
	if errors.As(err, &domainErr) {
		code := -32000
		switch domainErr.Code {
		case domain.ErrorInvalidInput:
			code = -32602
		case domain.ErrorNotFound:
			code = -32004
		case domain.ErrorConflict:
			code = -32009
		}
		return &RPCError{
			Code:    code,
			Message: domainErr.Message,
			Data:    &ErrorData{Code: string(domainErr.Code), Message: domainErr.Message},
		}
	}
	return &RPCError{Code: -32603, Message: "internal error"}
}

func pickProject(project string, fallback string) string {
	if strings.TrimSpace(project) == "" {
		return fallback
	}
	return project
}
