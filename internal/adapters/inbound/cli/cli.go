package cli

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	httpadapter "neabrain/internal/adapters/inbound/http"
	"neabrain/internal/adapters/inbound/mcp"
	"neabrain/internal/adapters/inbound/tui"
	"neabrain/internal/app"
	"neabrain/internal/domain"
	ports "neabrain/internal/ports/outbound"
)

// Run executes the CLI command with args and returns an exit code.
func Run(ctx context.Context, args []string, out io.Writer, errOut io.Writer) int {
	if out == nil {
		out = os.Stdout
	}
	if errOut == nil {
		errOut = os.Stderr
	}

	if len(args) == 0 || args[0] == "-h" || args[0] == "--help" || args[0] == "help" {
		writeUsage(out)
		return 2
	}

	switch args[0] {
	case "observation":
		return runObservation(ctx, args[1:], out, errOut)
	case "search":
		return runSearch(ctx, args[1:], out, errOut)
	case "topic":
		return runTopic(ctx, args[1:], out, errOut)
	case "session":
		return runSession(ctx, args[1:], out, errOut)
	case "config":
		return runConfig(ctx, args[1:], out, errOut)
	case "serve":
		return runServe(ctx, args[1:], out, errOut)
	case "mcp":
		return runMCP(ctx, args[1:], out, errOut)
	case "tui":
		return runTUI(ctx, args[1:], out, errOut)
	default:
		writeUsage(out)
		return 2
	}
}

func runObservation(ctx context.Context, args []string, out io.Writer, errOut io.Writer) int {
	if len(args) == 0 {
		writeObservationUsage(out)
		return 2
	}

	switch args[0] {
	case "create":
		return runObservationCreate(ctx, args[1:], out, errOut)
	case "read":
		return runObservationRead(ctx, args[1:], out, errOut)
	case "update":
		return runObservationUpdate(ctx, args[1:], out, errOut)
	case "list":
		return runObservationList(ctx, args[1:], out, errOut)
	case "delete":
		return runObservationDelete(ctx, args[1:], out, errOut)
	default:
		writeObservationUsage(out)
		return 2
	}
}

func runObservationCreate(ctx context.Context, args []string, out io.Writer, errOut io.Writer) int {
	fs := flag.NewFlagSet("observation create", flag.ContinueOnError)
	fs.SetOutput(errOut)

	var (
		content        string
		project        string
		topicKey       string
		tags           optionalCSV
		source         string
		metadata       optionalString
		allowDuplicate bool
		configFlags    configFlagSet
	)

	fs.StringVar(&content, "content", "", "Observation content")
	fs.StringVar(&project, "project", "", "Project name")
	fs.StringVar(&topicKey, "topic", "", "Topic key")
	fs.Var(&tags, "tags", "Comma-separated tags")
	fs.StringVar(&source, "source", "", "Observation source")
	fs.Var(&metadata, "metadata", "JSON metadata")
	fs.BoolVar(&allowDuplicate, "allow-duplicate", false, "Allow duplicates")
	configFlags.bind(fs)

	if err := fs.Parse(args); err != nil {
		return 2
	}

	overrides := configFlags.toOverrides()
	err := withApp(ctx, overrides, "observation.create", func(app *app.App) error {
		input := domain.ObservationCreateInput{
			Content:        content,
			Project:        pickProject(project, app.Config.DefaultProject),
			TopicKey:       topicKey,
			Tags:           tags.values,
			Source:         source,
			AllowDuplicate: allowDuplicate,
		}
		if metadata.set {
			parsed, err := parseMetadata(metadata.value)
			if err != nil {
				return err
			}
			input.Metadata = parsed
		}

		created, err := app.ObservationService.Create(ctx, input)
		if err != nil {
			return err
		}
		return writeJSON(out, created)
	})
	return handleError(err, errOut)
}

func runObservationRead(ctx context.Context, args []string, out io.Writer, errOut io.Writer) int {
	fs := flag.NewFlagSet("observation read", flag.ContinueOnError)
	fs.SetOutput(errOut)

	var (
		id             string
		includeDeleted bool
		configFlags    configFlagSet
	)
	fs.StringVar(&id, "id", "", "Observation id")
	fs.BoolVar(&includeDeleted, "include-deleted", false, "Include soft deleted observations")
	configFlags.bind(fs)

	if err := fs.Parse(args); err != nil {
		return 2
	}

	overrides := configFlags.toOverrides()
	err := withApp(ctx, overrides, "observation.read", func(app *app.App) error {
		observation, err := app.ObservationService.Read(ctx, id, includeDeleted)
		if err != nil {
			return err
		}
		return writeJSON(out, observation)
	})
	return handleError(err, errOut)
}

func runObservationUpdate(ctx context.Context, args []string, out io.Writer, errOut io.Writer) int {
	fs := flag.NewFlagSet("observation update", flag.ContinueOnError)
	fs.SetOutput(errOut)

	var (
		id          string
		content     optionalString
		project     optionalString
		topicKey    optionalString
		tags        optionalCSV
		source      optionalString
		metadata    optionalString
		configFlags configFlagSet
	)

	fs.StringVar(&id, "id", "", "Observation id")
	fs.Var(&content, "content", "Observation content")
	fs.Var(&project, "project", "Project name")
	fs.Var(&topicKey, "topic", "Topic key")
	fs.Var(&tags, "tags", "Comma-separated tags")
	fs.Var(&source, "source", "Observation source")
	fs.Var(&metadata, "metadata", "JSON metadata")
	configFlags.bind(fs)

	if err := fs.Parse(args); err != nil {
		return 2
	}

	overrides := configFlags.toOverrides()
	err := withApp(ctx, overrides, "observation.update", func(app *app.App) error {
		input := domain.ObservationUpdateInput{ID: id}
		if content.set {
			input.Content = &content.value
		}
		if project.set {
			projectValue := project.value
			input.Project = &projectValue
		}
		if topicKey.set {
			input.TopicKey = &topicKey.value
		}
		if tags.set {
			input.Tags = tags.values
		}
		if source.set {
			input.Source = &source.value
		}
		if metadata.set {
			parsed, err := parseMetadata(metadata.value)
			if err != nil {
				return err
			}
			input.Metadata = parsed
		}

		updated, err := app.ObservationService.Update(ctx, input)
		if err != nil {
			return err
		}
		return writeJSON(out, updated)
	})
	return handleError(err, errOut)
}

func runObservationList(ctx context.Context, args []string, out io.Writer, errOut io.Writer) int {
	fs := flag.NewFlagSet("observation list", flag.ContinueOnError)
	fs.SetOutput(errOut)

	var (
		project        string
		topicKey       string
		tags           optionalCSV
		includeDeleted bool
		configFlags    configFlagSet
	)
	fs.StringVar(&project, "project", "", "Project name")
	fs.StringVar(&topicKey, "topic", "", "Topic key")
	fs.Var(&tags, "tags", "Comma-separated tags")
	fs.BoolVar(&includeDeleted, "include-deleted", false, "Include soft deleted observations")
	configFlags.bind(fs)

	if err := fs.Parse(args); err != nil {
		return 2
	}

	overrides := configFlags.toOverrides()
	err := withApp(ctx, overrides, "observation.list", func(app *app.App) error {
		filter := domain.ObservationListFilter{
			Project:        pickProject(project, app.Config.DefaultProject),
			TopicKey:       topicKey,
			Tags:           tags.values,
			IncludeDeleted: includeDeleted,
		}
		observations, err := app.ObservationService.List(ctx, filter)
		if err != nil {
			return err
		}
		return writeJSON(out, observations)
	})
	return handleError(err, errOut)
}

func runObservationDelete(ctx context.Context, args []string, out io.Writer, errOut io.Writer) int {
	fs := flag.NewFlagSet("observation delete", flag.ContinueOnError)
	fs.SetOutput(errOut)

	var (
		id          string
		configFlags configFlagSet
	)
	fs.StringVar(&id, "id", "", "Observation id")
	configFlags.bind(fs)

	if err := fs.Parse(args); err != nil {
		return 2
	}

	overrides := configFlags.toOverrides()
	err := withApp(ctx, overrides, "observation.delete", func(app *app.App) error {
		deleted, err := app.ObservationService.SoftDelete(ctx, id)
		if err != nil {
			return err
		}
		return writeJSON(out, deleted)
	})
	return handleError(err, errOut)
}

func runSearch(ctx context.Context, args []string, out io.Writer, errOut io.Writer) int {
	fs := flag.NewFlagSet("search", flag.ContinueOnError)
	fs.SetOutput(errOut)

	var (
		query          string
		project        string
		topicKey       string
		tags           optionalCSV
		includeDeleted bool
		configFlags    configFlagSet
	)

	fs.StringVar(&query, "query", "", "Search query")
	fs.StringVar(&project, "project", "", "Project name")
	fs.StringVar(&topicKey, "topic", "", "Topic key")
	fs.Var(&tags, "tags", "Comma-separated tags")
	fs.BoolVar(&includeDeleted, "include-deleted", false, "Include soft deleted observations")
	configFlags.bind(fs)

	if err := fs.Parse(args); err != nil {
		return 2
	}

	overrides := configFlags.toOverrides()
	err := withApp(ctx, overrides, "search", func(app *app.App) error {
		filter := domain.SearchFilter{
			Project:        pickProject(project, app.Config.DefaultProject),
			TopicKey:       topicKey,
			Tags:           tags.values,
			IncludeDeleted: includeDeleted,
		}
		results, err := app.SearchService.Search(ctx, query, filter)
		if err != nil {
			return err
		}
		return writeJSON(out, results)
	})
	return handleError(err, errOut)
}

func runTopic(ctx context.Context, args []string, out io.Writer, errOut io.Writer) int {
	if len(args) == 0 || args[0] != "upsert" {
		writeTopicUsage(out)
		return 2
	}
	fs := flag.NewFlagSet("topic upsert", flag.ContinueOnError)
	fs.SetOutput(errOut)

	var (
		topicKey    string
		name        string
		description string
		metadata    optionalString
		configFlags configFlagSet
	)
	fs.StringVar(&topicKey, "topic", "", "Topic key")
	fs.StringVar(&name, "name", "", "Topic name")
	fs.StringVar(&description, "description", "", "Topic description")
	fs.Var(&metadata, "metadata", "JSON metadata")
	configFlags.bind(fs)

	if err := fs.Parse(args[1:]); err != nil {
		return 2
	}

	overrides := configFlags.toOverrides()
	err := withApp(ctx, overrides, "topic.upsert", func(app *app.App) error {
		input := domain.TopicUpsertInput{
			TopicKey:    topicKey,
			Name:        name,
			Description: description,
		}
		if metadata.set {
			parsed, err := parseMetadata(metadata.value)
			if err != nil {
				return err
			}
			input.Metadata = parsed
		}
		topic, err := app.TopicService.UpsertByTopicKey(ctx, input)
		if err != nil {
			return err
		}
		return writeJSON(out, topic)
	})
	return handleError(err, errOut)
}

func runSession(ctx context.Context, args []string, out io.Writer, errOut io.Writer) int {
	if len(args) == 0 {
		writeSessionUsage(out)
		return 2
	}

	switch args[0] {
	case "open":
		return runSessionOpen(ctx, args[1:], out, errOut)
	case "resume":
		return runSessionResume(ctx, args[1:], out, errOut)
	case "update-disclosure":
		return runSessionUpdateDisclosure(ctx, args[1:], out, errOut)
	default:
		writeSessionUsage(out)
		return 2
	}
}

func runSessionOpen(ctx context.Context, args []string, out io.Writer, errOut io.Writer) int {
	fs := flag.NewFlagSet("session open", flag.ContinueOnError)
	fs.SetOutput(errOut)

	var (
		disclosureLevel string
		configFlags     configFlagSet
	)
	fs.StringVar(&disclosureLevel, "disclosure-level", "", "Disclosure level")
	configFlags.bind(fs)

	if err := fs.Parse(args); err != nil {
		return 2
	}

	overrides := configFlags.toOverrides()
	err := withApp(ctx, overrides, "session.open", func(app *app.App) error {
		session, err := app.SessionService.Open(ctx, domain.SessionOpenInput{DisclosureLevel: disclosureLevel})
		if err != nil {
			return err
		}
		return writeJSON(out, session)
	})
	return handleError(err, errOut)
}

func runSessionResume(ctx context.Context, args []string, out io.Writer, errOut io.Writer) int {
	fs := flag.NewFlagSet("session resume", flag.ContinueOnError)
	fs.SetOutput(errOut)

	var (
		id          string
		configFlags configFlagSet
	)
	fs.StringVar(&id, "id", "", "Session id")
	configFlags.bind(fs)

	if err := fs.Parse(args); err != nil {
		return 2
	}

	overrides := configFlags.toOverrides()
	err := withApp(ctx, overrides, "session.resume", func(app *app.App) error {
		session, err := app.SessionService.Resume(ctx, id)
		if err != nil {
			return err
		}
		return writeJSON(out, session)
	})
	return handleError(err, errOut)
}

func runSessionUpdateDisclosure(ctx context.Context, args []string, out io.Writer, errOut io.Writer) int {
	fs := flag.NewFlagSet("session update-disclosure", flag.ContinueOnError)
	fs.SetOutput(errOut)

	var (
		id              string
		disclosureLevel string
		configFlags     configFlagSet
	)
	fs.StringVar(&id, "id", "", "Session id")
	fs.StringVar(&disclosureLevel, "disclosure-level", "", "Disclosure level")
	configFlags.bind(fs)

	if err := fs.Parse(args); err != nil {
		return 2
	}

	overrides := configFlags.toOverrides()
	err := withApp(ctx, overrides, "session.update_disclosure", func(app *app.App) error {
		session, err := app.SessionService.UpdateDisclosure(ctx, id, disclosureLevel)
		if err != nil {
			return err
		}
		return writeJSON(out, session)
	})
	return handleError(err, errOut)
}

func runConfig(ctx context.Context, args []string, out io.Writer, errOut io.Writer) int {
	if len(args) == 0 || args[0] != "show" {
		writeConfigUsage(out)
		return 2
	}

	fs := flag.NewFlagSet("config show", flag.ContinueOnError)
	fs.SetOutput(errOut)

	var configFlags configFlagSet
	configFlags.bind(fs)

	if err := fs.Parse(args[1:]); err != nil {
		return 2
	}

	overrides := configFlags.toOverrides()
	err := withApp(ctx, overrides, "config.show", func(app *app.App) error {
		return writeJSON(out, app.Config)
	})
	return handleError(err, errOut)
}

func runServe(ctx context.Context, args []string, out io.Writer, errOut io.Writer) int {
	fs := flag.NewFlagSet("serve", flag.ContinueOnError)
	fs.SetOutput(errOut)

	var (
		addr        string
		configFlags configFlagSet
	)
	fs.StringVar(&addr, "addr", ":8080", "HTTP listen address")
	configFlags.bind(fs)

	if err := fs.Parse(args); err != nil {
		return 2
	}

	overrides := configFlags.toOverrides()
	appInstance, err := app.Bootstrap(ctx, overrides)
	if err != nil {
		return handleError(err, errOut)
	}
	defer func() {
		_ = appInstance.Close()
	}()

	appInstance.Logger.Info("http server start", map[string]any{"addr": addr})
	appInstance.Metrics.Inc("adapter.http.listen")
	server := httpadapter.NewServer(appInstance, addr)
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return handleError(err, errOut)
	}
	return 0
}

func runMCP(ctx context.Context, args []string, out io.Writer, errOut io.Writer) int {
	fs := flag.NewFlagSet("mcp", flag.ContinueOnError)
	fs.SetOutput(errOut)

	var configFlags configFlagSet
	configFlags.bind(fs)

	if err := fs.Parse(args); err != nil {
		return 2
	}

	overrides := configFlags.toOverrides()
	appInstance, err := app.Bootstrap(ctx, overrides)
	if err != nil {
		return handleError(err, errOut)
	}
	defer func() {
		_ = appInstance.Close()
	}()

	appInstance.Logger.Info("mcp server start", nil)
	appInstance.Metrics.Inc("adapter.mcp.listen")
	server := mcp.NewServer(appInstance)
	if err := server.Serve(ctx, os.Stdin, out); err != nil {
		return handleError(err, errOut)
	}
	return 0
}

func runTUI(ctx context.Context, args []string, out io.Writer, errOut io.Writer) int {
	return tui.Run(ctx, args, os.Stdin, out, errOut, Run)
}

func withApp(ctx context.Context, overrides ports.ConfigOverrides, operation string, fn func(*app.App) error) error {
	appInstance, err := app.Bootstrap(ctx, overrides)
	if err != nil {
		return err
	}
	defer func() {
		_ = appInstance.Close()
	}()
	if strings.TrimSpace(operation) != "" {
		appInstance.Logger.Info("cli command", map[string]any{"operation": operation})
		appInstance.Metrics.Inc("adapter.cli." + operation)
	}
	return fn(appInstance)
}

func handleError(err error, errOut io.Writer) int {
	if err == nil {
		return 0
	}

	fmt.Fprintln(errOut, err.Error())
	return exitCodeForError(err)
}

func exitCodeForError(err error) int {
	var domainErr domain.DomainError
	if errors.As(err, &domainErr) {
		switch domainErr.Code {
		case domain.ErrorInvalidInput:
			return 2
		case domain.ErrorNotFound:
			return 3
		case domain.ErrorConflict:
			return 4
		default:
			return 1
		}
	}
	return 1
}

type configFlagSet struct {
	storagePath    optionalString
	ftsPath        optionalString
	defaultProject optionalString
	dedupePolicy   optionalString
	configFile     optionalString
}

func (c *configFlagSet) bind(fs *flag.FlagSet) {
	fs.Var(&c.storagePath, "storage-path", "Storage path override")
	fs.Var(&c.ftsPath, "fts-path", "FTS path override")
	fs.Var(&c.defaultProject, "default-project", "Default project override")
	fs.Var(&c.dedupePolicy, "dedupe-policy", "Dedupe policy override (exact|none)")
	fs.Var(&c.configFile, "config-file", "Config file override")
}

func (c *configFlagSet) toOverrides() ports.ConfigOverrides {
	return ports.ConfigOverrides{
		StoragePath:    optionalStringPtr(c.storagePath),
		FTSPath:        optionalStringPtr(c.ftsPath),
		DefaultProject: optionalStringPtr(c.defaultProject),
		DedupePolicy:   optionalStringPtr(c.dedupePolicy),
		ConfigFile:     optionalStringPtr(c.configFile),
	}
}

type optionalString struct {
	set   bool
	value string
}

func (o *optionalString) String() string {
	return o.value
}

func (o *optionalString) Set(val string) error {
	o.value = val
	o.set = true
	return nil
}

type optionalCSV struct {
	set    bool
	values []string
}

func (o *optionalCSV) String() string {
	return strings.Join(o.values, ",")
}

func (o *optionalCSV) Set(val string) error {
	o.set = true
	o.values = parseCSV(val)
	return nil
}

func parseCSV(val string) []string {
	trimmed := strings.TrimSpace(val)
	if trimmed == "" {
		return []string{}
	}
	parts := strings.Split(trimmed, ",")
	values := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmedPart := strings.TrimSpace(part)
		if trimmedPart != "" {
			values = append(values, trimmedPart)
		}
	}
	return values
}

func optionalStringPtr(val optionalString) *string {
	if !val.set {
		return nil
	}
	value := val.value
	return &value
}

func parseMetadata(raw string) (map[string]any, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return map[string]any{}, nil
	}
	var parsed map[string]any
	if err := json.Unmarshal([]byte(trimmed), &parsed); err != nil {
		return nil, fmt.Errorf("invalid metadata JSON: %w", err)
	}
	return parsed, nil
}

func pickProject(project string, fallback string) string {
	if strings.TrimSpace(project) == "" {
		return fallback
	}
	return project
}

func writeJSON(out io.Writer, payload any) error {
	data, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return err
	}
	_, err = fmt.Fprintln(out, string(data))
	return err
}

func writeUsage(out io.Writer) {
	fmt.Fprintln(out, "neabrain <command> [args]")
	fmt.Fprintln(out, "")
	fmt.Fprintln(out, "Commands:")
	fmt.Fprintln(out, "  observation <create|read|update|list|delete>")
	fmt.Fprintln(out, "  search")
	fmt.Fprintln(out, "  topic upsert")
	fmt.Fprintln(out, "  session <open|resume|update-disclosure>")
	fmt.Fprintln(out, "  config show")
	fmt.Fprintln(out, "  serve")
	fmt.Fprintln(out, "  mcp")
	fmt.Fprintln(out, "  tui")
}

func writeObservationUsage(out io.Writer) {
	fmt.Fprintln(out, "neabrain observation <create|read|update|list|delete>")
}

func writeTopicUsage(out io.Writer) {
	fmt.Fprintln(out, "neabrain topic upsert")
}

func writeSessionUsage(out io.Writer) {
	fmt.Fprintln(out, "neabrain session <open|resume|update-disclosure>")
}

func writeConfigUsage(out io.Writer) {
	fmt.Fprintln(out, "neabrain config show")
}
