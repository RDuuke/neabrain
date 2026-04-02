package domain

// ObservationCreateInput captures fields required to create an observation.
type ObservationCreateInput struct {
	Content        string
	Project        string
	TopicKey       string
	Tags           []string
	Source         string
	Metadata       map[string]any
	AllowDuplicate bool
}

// ObservationUpdateInput captures fields required to update an observation.
type ObservationUpdateInput struct {
	ID       string
	Content  *string
	Project  *string
	TopicKey *string
	Tags     []string
	Source   *string
	Metadata map[string]any
}

// ObservationListFilter constrains observation listings.
type ObservationListFilter struct {
	Project         string
	TopicKey        string
	Tags            []string
	IncludeDeleted  bool
	DisclosureLevel string
}

// SearchFilter constrains search results.
type SearchFilter struct {
	Project         string
	TopicKey        string
	Tags            []string
	IncludeDeleted  bool
	DisclosureLevel string
}

// TopicUpsertInput captures fields required to upsert a topic.
type TopicUpsertInput struct {
	TopicKey    string
	Name        string
	Description string
	Metadata    map[string]any
}

// SessionOpenInput captures fields required to open a session.
type SessionOpenInput struct {
	DisclosureLevel string
}
