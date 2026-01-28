package contract

// NotificationEvent is the event envelope sent into Kafka (or HTTP webhook).
// Identity determines handler and channels; payload follows BaseEvent (RDF-like).
type NotificationEvent struct {
	Identity string      `json:"identity"`           // e.g. "content_published", "comment_created"
	Payload  BaseEvent   `json:"payload"`             // actor, do, io?, po?
	Metadata *EventMetadata `json:"metadata,omitempty"` // tracing (e.g. x-request-id)
	Extra    *EventExtra `json:"extra,omitempty"`    // to, ignore, old_data, custom fields
}

// EventMetadata holds optional tracing/metadata (e.g. request-id).
type EventMetadata struct {
	RequestID string `json:"x-request-id,omitempty"`
}

// EventExtra holds optional fields: to (recipients), ignore, old_data, and arbitrary extras.
type EventExtra struct {
	To      interface{}            `json:"to,omitempty"`      // recipient ids or filter
	Ignore  interface{}            `json:"ignore,omitempty"`  // ids to exclude
	OldData interface{}            `json:"old_data,omitempty"` // previous state if update
	Rest    map[string]interface{} `json:"-"`                // other fields via custom unmarshal or separate map
}
