package httpadapter

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"neabrain/internal/app"
	"neabrain/internal/domain"
	"neabrain/internal/observability"
)

// NewServer builds an HTTP server for core operations.
func NewServer(appInstance *app.App, addr string) *http.Server {
	return &http.Server{
		Addr:    addr,
		Handler: NewHandler(appInstance),
	}
}

// NewServerWithNotifier builds an HTTP server with an optional post-write notifier.
func NewServerWithNotifier(appInstance *app.App, addr string, notifier WriteNotifier) *http.Server {
	return &http.Server{
		Addr:    addr,
		Handler: NewHandlerWithNotifier(appInstance, notifier),
	}
}

// WriteNotifier is called in the background after each successful write mutation.
// A nil notifier is a no-op.
type WriteNotifier func()

// NewHandler returns a mux with all API routes.
func NewHandler(appInstance *app.App) http.Handler {
	return NewHandlerWithNotifier(appInstance, nil)
}

// NewHandlerWithNotifier returns a mux with an optional post-write notifier.
func NewHandlerWithNotifier(appInstance *app.App, notifier WriteNotifier) http.Handler {
	handler := &Handler{app: appInstance, notifier: notifier}
	mux := http.NewServeMux()
	mux.HandleFunc("/observations", handler.handleObservations)
	mux.HandleFunc("/observations/", handler.handleObservationByID)
	mux.HandleFunc("/search", handler.handleSearch)
	mux.HandleFunc("/topics/", handler.handleTopicByKey)
	mux.HandleFunc("/sessions", handler.handleSessions)
	mux.HandleFunc("/sessions/", handler.handleSessionByID)
	return &loggingHandler{next: mux, logger: appInstance.Logger, metrics: appInstance.Metrics}
}

// Handler maps HTTP requests to domain services.
type Handler struct {
	app      *app.App
	notifier WriteNotifier
}

func (h *Handler) notifyWrite() {
	if h.notifier != nil {
		go h.notifier()
	}
}

type loggingHandler struct {
	next    http.Handler
	logger  *observability.Logger
	metrics *observability.Metrics
}

func (h *loggingHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.next == nil {
		return
	}
	if h.logger != nil {
		h.logger.Info("http request", map[string]any{"method": r.Method, "path": r.URL.Path})
	}
	if h.metrics != nil {
		h.metrics.Inc("adapter.http.request")
	}
	h.next.ServeHTTP(w, r)
}

type errorPayload struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type errorResponse struct {
	Error errorPayload `json:"error"`
}

type observationCreateRequest struct {
	Content        string         `json:"content"`
	Project        string         `json:"project"`
	TopicKey       string         `json:"topic_key"`
	Tags           []string       `json:"tags"`
	Source         string         `json:"source"`
	Metadata       map[string]any `json:"metadata"`
	AllowDuplicate bool           `json:"allow_duplicate"`
}

type observationUpdateRequest struct {
	Content  *string         `json:"content"`
	Project  *string         `json:"project"`
	TopicKey *string         `json:"topic_key"`
	Tags     *[]string       `json:"tags"`
	Source   *string         `json:"source"`
	Metadata *map[string]any `json:"metadata"`
}

type topicUpsertRequest struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Metadata    map[string]any `json:"metadata"`
}

type sessionOpenRequest struct {
	DisclosureLevel string `json:"disclosure_level"`
}

type sessionUpdateRequest struct {
	DisclosureLevel string `json:"disclosure_level"`
}

func (h *Handler) handleObservations(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
		h.createObservation(w, r)
	case http.MethodGet:
		h.listObservations(w, r)
	default:
		writeMethodNotAllowed(w)
	}
}

func (h *Handler) handleObservationByID(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/observations/")
	if id == "" || strings.Contains(id, "/") {
		writeNotFound(w)
		return
	}

	switch r.Method {
	case http.MethodGet:
		h.readObservation(w, r, id)
	case http.MethodPatch:
		h.updateObservation(w, r, id)
	case http.MethodDelete:
		h.deleteObservation(w, r, id)
	default:
		writeMethodNotAllowed(w)
	}
}

func (h *Handler) handleSearch(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeMethodNotAllowed(w)
		return
	}

	query := strings.TrimSpace(r.URL.Query().Get("query"))
	if query == "" {
		query = strings.TrimSpace(r.URL.Query().Get("q"))
	}

	filter := domain.SearchFilter{
		Project:        pickProject(r.URL.Query().Get("project"), h.app.Config.DefaultProject),
		TopicKey:       r.URL.Query().Get("topic_key"),
		Tags:           parseTagsQuery(r.URL.Query().Get("tags")),
		IncludeDeleted: parseBool(r.URL.Query().Get("include_deleted")),
	}
	disclosureLevel, err := h.resolveDisclosureLevel(r)
	if err != nil {
		writeError(w, err)
		return
	}
	filter.DisclosureLevel = disclosureLevel

	results, err := h.app.SearchService.Search(r.Context(), query, filter)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, results)
}

func (h *Handler) handleTopicByKey(w http.ResponseWriter, r *http.Request) {
	topicKey := strings.TrimPrefix(r.URL.Path, "/topics/")
	if topicKey == "" || strings.Contains(topicKey, "/") {
		writeNotFound(w)
		return
	}

	if r.Method != http.MethodPut {
		writeMethodNotAllowed(w)
		return
	}

	var req topicUpsertRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, err)
		return
	}

	topic, err := h.app.TopicService.UpsertByTopicKey(r.Context(), domain.TopicUpsertInput{
		TopicKey:    topicKey,
		Name:        req.Name,
		Description: req.Description,
		Metadata:    req.Metadata,
	})
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, topic)
}

func (h *Handler) handleSessions(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeMethodNotAllowed(w)
		return
	}

	var req sessionOpenRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, err)
		return
	}

	session, err := h.app.SessionService.Open(r.Context(), domain.SessionOpenInput{DisclosureLevel: req.DisclosureLevel})
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, session)
}

func (h *Handler) handleSessionByID(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/sessions/")
	if path == "" {
		writeNotFound(w)
		return
	}

	parts := strings.Split(path, "/")
	if len(parts) == 0 || parts[0] == "" {
		writeNotFound(w)
		return
	}
	sessionID := parts[0]

	if len(parts) == 2 && parts[1] == "resume" {
		if r.Method != http.MethodPost {
			writeMethodNotAllowed(w)
			return
		}
		session, err := h.app.SessionService.Resume(r.Context(), sessionID)
		if err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, session)
		return
	}

	if len(parts) == 1 && r.Method == http.MethodPatch {
		var req sessionUpdateRequest
		if err := decodeJSON(r, &req); err != nil {
			writeError(w, err)
			return
		}
		session, err := h.app.SessionService.UpdateDisclosure(r.Context(), sessionID, req.DisclosureLevel)
		if err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, session)
		return
	}

	writeMethodNotAllowed(w)
}

func (h *Handler) createObservation(w http.ResponseWriter, r *http.Request) {
	var req observationCreateRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, err)
		return
	}

	created, err := h.app.ObservationService.Create(r.Context(), domain.ObservationCreateInput{
		Content:        req.Content,
		Project:        pickProject(req.Project, h.app.Config.DefaultProject),
		TopicKey:       req.TopicKey,
		Tags:           req.Tags,
		Source:         req.Source,
		Metadata:       req.Metadata,
		AllowDuplicate: req.AllowDuplicate,
	})
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, created)
	h.notifyWrite()
}

func (h *Handler) readObservation(w http.ResponseWriter, r *http.Request, id string) {
	includeDeleted := parseBool(r.URL.Query().Get("include_deleted"))
	observation, err := h.app.ObservationService.Read(r.Context(), id, includeDeleted)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, observation)
}

func (h *Handler) updateObservation(w http.ResponseWriter, r *http.Request, id string) {
	var req observationUpdateRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, err)
		return
	}

	input := domain.ObservationUpdateInput{ID: id}
	if req.Content != nil {
		input.Content = req.Content
	}
	if req.Project != nil {
		input.Project = req.Project
	}
	if req.TopicKey != nil {
		input.TopicKey = req.TopicKey
	}
	if req.Tags != nil {
		input.Tags = *req.Tags
	}
	if req.Source != nil {
		input.Source = req.Source
	}
	if req.Metadata != nil {
		input.Metadata = *req.Metadata
	}

	updated, err := h.app.ObservationService.Update(r.Context(), input)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, updated)
	h.notifyWrite()
}

func (h *Handler) listObservations(w http.ResponseWriter, r *http.Request) {
	filter := domain.ObservationListFilter{
		Project:        pickProject(r.URL.Query().Get("project"), h.app.Config.DefaultProject),
		TopicKey:       r.URL.Query().Get("topic_key"),
		Tags:           parseTagsQuery(r.URL.Query().Get("tags")),
		IncludeDeleted: parseBool(r.URL.Query().Get("include_deleted")),
	}
	disclosureLevel, err := h.resolveDisclosureLevel(r)
	if err != nil {
		writeError(w, err)
		return
	}
	filter.DisclosureLevel = disclosureLevel
	observations, err := h.app.ObservationService.List(r.Context(), filter)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, observations)
}

func (h *Handler) deleteObservation(w http.ResponseWriter, r *http.Request, id string) {
	deleted, err := h.app.ObservationService.SoftDelete(r.Context(), id)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, deleted)
	h.notifyWrite()
}

func decodeJSON(r *http.Request, target any) error {
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return domain.NewInvalidInput("invalid JSON body")
	}
	return nil
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	data, err := json.Marshal(payload)
	if err != nil {
		writeError(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = w.Write(data)
}

func writeError(w http.ResponseWriter, err error) {
	status := http.StatusInternalServerError
	code := "internal_error"
	message := err.Error()

	var domainErr domain.DomainError
	if errors.As(err, &domainErr) {
		code = string(domainErr.Code)
		message = domainErr.Message
		switch domainErr.Code {
		case domain.ErrorInvalidInput:
			status = http.StatusBadRequest
		case domain.ErrorNotFound:
			status = http.StatusNotFound
		case domain.ErrorConflict:
			status = http.StatusConflict
		}
	}

	writeJSON(w, status, errorResponse{Error: errorPayload{Code: code, Message: message}})
}

func writeMethodNotAllowed(w http.ResponseWriter) {
	w.WriteHeader(http.StatusMethodNotAllowed)
}

func writeNotFound(w http.ResponseWriter) {
	w.WriteHeader(http.StatusNotFound)
}

func parseBool(value string) bool {
	trimmed := strings.ToLower(strings.TrimSpace(value))
	return trimmed == "true" || trimmed == "1" || trimmed == "yes"
}

func parseTagsQuery(value string) []string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return []string{}
	}
	parts := strings.Split(trimmed, ",")
	tags := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmedPart := strings.TrimSpace(part)
		if trimmedPart != "" {
			tags = append(tags, trimmedPart)
		}
	}
	return tags
}

func pickProject(project string, fallback string) string {
	if strings.TrimSpace(project) == "" {
		return fallback
	}
	return project
}

func (h *Handler) resolveDisclosureLevel(r *http.Request) (string, error) {
	sessionID := strings.TrimSpace(r.URL.Query().Get("session_id"))
	if sessionID != "" {
		session, err := h.app.SessionService.Read(r.Context(), sessionID)
		if err != nil {
			return "", err
		}
		return session.DisclosureLevel, nil
	}
	return strings.TrimSpace(r.URL.Query().Get("disclosure_level")), nil
}
