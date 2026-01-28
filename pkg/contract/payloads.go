package contract

// ContentPublishedPayload: actor (user) publishes content; do = post/article; io/po optional.
// Use BaseEvent with Do[0].Type in (post, article) for type-safe handling.
type ContentPublishedPayload struct {
	BaseEvent
}

// CommentCreatedPayload: actor (user) creates comment; do = comment; io = post/article/comment parent; po = group.
type CommentCreatedPayload struct {
	BaseEvent
}
