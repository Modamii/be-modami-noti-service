package contract

import (
	"errors"
	"fmt"
)

var (
	ErrMissingIdentity  = errors.New("contract: identity is required")
	ErrMissingActor     = errors.New("contract: actor.id and actor.type are required")
	ErrMissingDo        = errors.New("contract: at least one do object is required")
	ErrIncompleteObject = errors.New("contract: object must have id and type")
	ErrUnknownIdentity  = errors.New("contract: unknown identity")
)

// NotificationEvent is the event envelope sent into Kafka (or HTTP webhook).
//
// Grammar (RDF-like):
//
//	Identity: what happened (verb)       — "content_published", "comment_created"
//	Payload:  who did what to whom       — actor (subject), do (direct object), io (indirect), po (prepositional)
//	Metadata: tracing / correlation      — request-id, timestamp, source
//	Extra:    routing & enrichment hints — recipients (to), exclusions (ignore), previous state (old_data)
//
// Example:
//
//	{
//	  "identity": "comment_created",
//	  "payload": {
//	    "actor": { "id": "user-1", "type": "user", "data": {"name": "Alice"} },
//	    "do":    [{ "id": "cmt-1", "type": "comment", "data": {"content": "Great post!"} }],
//	    "io":    [{ "id": "post-1", "type": "post",   "data": {"title": "My Article"} }]
//	  },
//	  "extra": { "to": ["user-2", "user-3"] }
//	}
type NotificationEvent struct {
	Identity string         `json:"identity"`            // e.g. "content_published", "comment_created"
	Payload  BaseEvent      `json:"payload"`             // actor, do, io?, po?
	Metadata *EventMetadata `json:"metadata,omitempty"`  // tracing (e.g. x-request-id)
	Extra    *EventExtra    `json:"extra,omitempty"`     // to, ignore, old_data, custom fields
}

// Validate checks that the event envelope has the minimum required fields.
func (e *NotificationEvent) Validate() error {
	if e.Identity == "" {
		return ErrMissingIdentity
	}
	if e.Payload.Actor.ID == "" || e.Payload.Actor.Type == "" {
		return ErrMissingActor
	}
	if len(e.Payload.Do) == 0 {
		return ErrMissingDo
	}
	for i, obj := range e.Payload.Do {
		if obj.ID == "" || obj.Type == "" {
			return fmt.Errorf("%w: do[%d]", ErrIncompleteObject, i)
		}
	}
	if !IsValidIdentity(e.Identity) {
		return fmt.Errorf("%w: %s", ErrUnknownIdentity, e.Identity)
	}
	return nil
}

// EventMetadata holds optional tracing/metadata (e.g. request-id).
type EventMetadata struct {
	RequestID string `json:"x-request-id,omitempty"`
	Source    string `json:"source,omitempty"`    // originating service
	Timestamp int64  `json:"timestamp,omitempty"` // unix millis
}

// EventExtra holds optional fields: to (recipients), ignore, old_data, and arbitrary extras.
type EventExtra struct {
	To      interface{}            `json:"to,omitempty"`       // recipient ids or filter
	Ignore  interface{}            `json:"ignore,omitempty"`   // ids to exclude
	OldData interface{}            `json:"old_data,omitempty"` // previous state if update
	Rest    map[string]interface{} `json:"-"`                  // other fields via custom unmarshal or separate map
}
