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

// Server handles MCP-style tool requests over JSON-RPC.
type Server struct {
	app *app.App
}

// NewServer constructs a MCP server bound to the app services.
func NewServer(appInstance *app.App) *Server {
	return &Server{app: appInstance}
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
		return Response{JSONRPC: "2.0", ID: req.ID, Result: toolsListResult{Tools: toolDefinitions()}}
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

type ToolDefinition struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	InputSchema map[string]any `json:"inputSchema"`
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

func toolDefinitions() []ToolDefinition {
	return []ToolDefinition{
		{Name: "observation.create", Description: "Create an observation", InputSchema: schemaObject(map[string]any{
			"content":         schemaString(),
			"project":         schemaString(),
			"topic_key":       schemaString(),
			"tags":            schemaStringArray(),
			"source":          schemaString(),
			"metadata":        schemaObjectAny(),
			"allow_duplicate": schemaBool(),
		}, "content")},
		{Name: "nbn_observation_create", Description: "Alias for observation.create", InputSchema: schemaObject(map[string]any{
			"content":         schemaString(),
			"project":         schemaString(),
			"topic_key":       schemaString(),
			"tags":            schemaStringArray(),
			"source":          schemaString(),
			"metadata":        schemaObjectAny(),
			"allow_duplicate": schemaBool(),
		}, "content")},
		{Name: "observation.read", Description: "Read an observation by id", InputSchema: schemaObject(map[string]any{
			"id":              schemaString(),
			"include_deleted": schemaBool(),
		}, "id")},
		{Name: "nbn_observation_read", Description: "Alias for observation.read", InputSchema: schemaObject(map[string]any{
			"id":              schemaString(),
			"include_deleted": schemaBool(),
		}, "id")},
		{Name: "observation.update", Description: "Update an observation", InputSchema: schemaObject(map[string]any{
			"id":        schemaString(),
			"content":   schemaString(),
			"project":   schemaString(),
			"topic_key": schemaString(),
			"tags":      schemaStringArray(),
			"source":    schemaString(),
			"metadata":  schemaObjectAny(),
		}, "id")},
		{Name: "nbn_observation_update", Description: "Alias for observation.update", InputSchema: schemaObject(map[string]any{
			"id":        schemaString(),
			"content":   schemaString(),
			"project":   schemaString(),
			"topic_key": schemaString(),
			"tags":      schemaStringArray(),
			"source":    schemaString(),
			"metadata":  schemaObjectAny(),
		}, "id")},
		{Name: "observation.list", Description: "List observations", InputSchema: schemaObject(map[string]any{
			"project":         schemaString(),
			"topic_key":       schemaString(),
			"tags":            schemaStringArray(),
			"include_deleted": schemaBool(),
		})},
		{Name: "nbn_observation_list", Description: "Alias for observation.list", InputSchema: schemaObject(map[string]any{
			"project":         schemaString(),
			"topic_key":       schemaString(),
			"tags":            schemaStringArray(),
			"include_deleted": schemaBool(),
		})},
		{Name: "observation.delete", Description: "Soft delete an observation", InputSchema: schemaObject(map[string]any{
			"id": schemaString(),
		}, "id")},
		{Name: "nbn_observation_delete", Description: "Alias for observation.delete", InputSchema: schemaObject(map[string]any{
			"id": schemaString(),
		}, "id")},
		{Name: "search", Description: "Search observations", InputSchema: schemaObject(map[string]any{
			"query":           schemaString(),
			"project":         schemaString(),
			"topic_key":       schemaString(),
			"tags":            schemaStringArray(),
			"include_deleted": schemaBool(),
		}, "query")},
		{Name: "nbn_search", Description: "Alias for search", InputSchema: schemaObject(map[string]any{
			"query":           schemaString(),
			"project":         schemaString(),
			"topic_key":       schemaString(),
			"tags":            schemaStringArray(),
			"include_deleted": schemaBool(),
		}, "query")},
		{Name: "nbn_context", Description: "Alias for search (context)", InputSchema: schemaObject(map[string]any{
			"query":           schemaString(),
			"project":         schemaString(),
			"topic_key":       schemaString(),
			"tags":            schemaStringArray(),
			"include_deleted": schemaBool(),
		}, "query")},
		{Name: "topic.upsert", Description: "Upsert a topic by key", InputSchema: schemaObject(map[string]any{
			"topic_key":   schemaString(),
			"name":        schemaString(),
			"description": schemaString(),
			"metadata":    schemaObjectAny(),
		}, "topic_key")},
		{Name: "nbn_topic_upsert", Description: "Alias for topic.upsert", InputSchema: schemaObject(map[string]any{
			"topic_key":   schemaString(),
			"name":        schemaString(),
			"description": schemaString(),
			"metadata":    schemaObjectAny(),
		}, "topic_key")},
		{Name: "session.open", Description: "Open a session", InputSchema: schemaObject(map[string]any{
			"disclosure_level": schemaString(),
		}, "disclosure_level")},
		{Name: "nbn_session_open", Description: "Alias for session.open", InputSchema: schemaObject(map[string]any{
			"disclosure_level": schemaString(),
		}, "disclosure_level")},
		{Name: "session.resume", Description: "Resume a session", InputSchema: schemaObject(map[string]any{
			"id": schemaString(),
		}, "id")},
		{Name: "nbn_session_resume", Description: "Alias for session.resume", InputSchema: schemaObject(map[string]any{
			"id": schemaString(),
		}, "id")},
		{Name: "session.update_disclosure", Description: "Update disclosure level", InputSchema: schemaObject(map[string]any{
			"id":               schemaString(),
			"disclosure_level": schemaString(),
		}, "id", "disclosure_level")},
		{Name: "nbn_session_update_disclosure", Description: "Alias for session.update_disclosure", InputSchema: schemaObject(map[string]any{
			"id":               schemaString(),
			"disclosure_level": schemaString(),
		}, "id", "disclosure_level")},
		{Name: "config.show", Description: "Show effective config", InputSchema: schemaObject(map[string]any{})},
		{Name: "nbn_config_show", Description: "Alias for config.show", InputSchema: schemaObject(map[string]any{})},
		{Name: "nbn_session_summary", Description: "Store a session summary", InputSchema: schemaObject(map[string]any{
			"summary":   schemaString(),
			"project":   schemaString(),
			"topic_key": schemaString(),
			"tags":      schemaStringArray(),
			"metadata":  schemaObjectAny(),
		}, "summary")},
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
