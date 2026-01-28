package contract

// EventObject is the unified entity shape: id, type, optional data.
// Every entity in actor, do, io, po uses this format.
type EventObject struct {
	ID   string                 `json:"id"`
	Type string                 `json:"type"` // ObjectType: user, post, article, comment, group, system, ...
	Data map[string]interface{} `json:"data,omitempty"`
}

// ObjectType constants (used in EventObject.Type).
const (
	ObjectTypeUser    = "user"
	ObjectTypeSystem  = "system"
	ObjectTypePost    = "post"
	ObjectTypeArticle = "article"
	ObjectTypeComment = "comment"
	ObjectTypeGroup   = "group"
)
