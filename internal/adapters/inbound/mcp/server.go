package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"

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
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments"`
}

type toolsListResult struct {
	Tools []ToolDefinition `json:"tools"`
}

type ToolDefinition struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	InputSchema map[string]any `json:"input_schema"`
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

func (s *Server) handleToolsCall(ctx context.Context, req Request) Response {
	var params toolsCallParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return Response{JSONRPC: "2.0", ID: req.ID, Error: &RPCError{Code: -32602, Message: "invalid params"}}
	}
	name := strings.TrimSpace(params.Name)
	if name == "" {
		return Response{JSONRPC: "2.0", ID: req.ID, Error: &RPCError{Code: -32602, Message: "tool name required"}}
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
	case "observation.create":
		var args observationCreateArgs
		if err := json.Unmarshal(params.Arguments, &args); err != nil {
			return Response{JSONRPC: "2.0", ID: req.ID, Error: &RPCError{Code: -32602, Message: "invalid observation.create args"}}
		}
		created, err := s.app.ObservationService.Create(ctx, domain.ObservationCreateInput{
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
		observation, err := s.app.ObservationService.Read(ctx, args.ID, args.IncludeDeleted)
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
		updated, err := s.app.ObservationService.Update(ctx, input)
		if err != nil {
			return Response{JSONRPC: "2.0", ID: req.ID, Error: rpcErrorFrom(err)}
		}
		return Response{JSONRPC: "2.0", ID: req.ID, Result: updated}
	case "observation.list":
		var args observationListArgs
		if err := json.Unmarshal(params.Arguments, &args); err != nil {
			return Response{JSONRPC: "2.0", ID: req.ID, Error: &RPCError{Code: -32602, Message: "invalid observation.list args"}}
		}
		observations, err := s.app.ObservationService.List(ctx, domain.ObservationListFilter{
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
		deleted, err := s.app.ObservationService.SoftDelete(ctx, args.ID)
		if err != nil {
			return Response{JSONRPC: "2.0", ID: req.ID, Error: rpcErrorFrom(err)}
		}
		return Response{JSONRPC: "2.0", ID: req.ID, Result: deleted}
	case "search":
		var args searchArgs
		if err := json.Unmarshal(params.Arguments, &args); err != nil {
			return Response{JSONRPC: "2.0", ID: req.ID, Error: &RPCError{Code: -32602, Message: "invalid search args"}}
		}
		results, err := s.app.SearchService.Search(ctx, args.Query, domain.SearchFilter{
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
		topic, err := s.app.TopicService.UpsertByTopicKey(ctx, domain.TopicUpsertInput{
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
		session, err := s.app.SessionService.Open(ctx, domain.SessionOpenInput{DisclosureLevel: args.DisclosureLevel})
		if err != nil {
			return Response{JSONRPC: "2.0", ID: req.ID, Error: rpcErrorFrom(err)}
		}
		return Response{JSONRPC: "2.0", ID: req.ID, Result: session}
	case "session.resume":
		var args sessionResumeArgs
		if err := json.Unmarshal(params.Arguments, &args); err != nil {
			return Response{JSONRPC: "2.0", ID: req.ID, Error: &RPCError{Code: -32602, Message: "invalid session.resume args"}}
		}
		session, err := s.app.SessionService.Resume(ctx, args.ID)
		if err != nil {
			return Response{JSONRPC: "2.0", ID: req.ID, Error: rpcErrorFrom(err)}
		}
		return Response{JSONRPC: "2.0", ID: req.ID, Result: session}
	case "session.update_disclosure":
		var args sessionUpdateArgs
		if err := json.Unmarshal(params.Arguments, &args); err != nil {
			return Response{JSONRPC: "2.0", ID: req.ID, Error: &RPCError{Code: -32602, Message: "invalid session.update_disclosure args"}}
		}
		session, err := s.app.SessionService.UpdateDisclosure(ctx, args.ID, args.DisclosureLevel)
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
		{Name: "observation.read", Description: "Read an observation by id", InputSchema: schemaObject(map[string]any{
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
		{Name: "observation.list", Description: "List observations", InputSchema: schemaObject(map[string]any{
			"project":         schemaString(),
			"topic_key":       schemaString(),
			"tags":            schemaStringArray(),
			"include_deleted": schemaBool(),
		})},
		{Name: "observation.delete", Description: "Soft delete an observation", InputSchema: schemaObject(map[string]any{
			"id": schemaString(),
		}, "id")},
		{Name: "search", Description: "Search observations", InputSchema: schemaObject(map[string]any{
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
		{Name: "session.open", Description: "Open a session", InputSchema: schemaObject(map[string]any{
			"disclosure_level": schemaString(),
		}, "disclosure_level")},
		{Name: "session.resume", Description: "Resume a session", InputSchema: schemaObject(map[string]any{
			"id": schemaString(),
		}, "id")},
		{Name: "session.update_disclosure", Description: "Update disclosure level", InputSchema: schemaObject(map[string]any{
			"id":               schemaString(),
			"disclosure_level": schemaString(),
		}, "id", "disclosure_level")},
		{Name: "config.show", Description: "Show effective config", InputSchema: schemaObject(map[string]any{})},
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
