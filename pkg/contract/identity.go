package contract

// Identity constants (string values used in Kafka message value).
// Producers and notification service share this set.
const (
	ContentPublished = "content_published"
	CommentCreated   = "comment_created"
)

// AllIdentities lists every identity for validation/documentation.
var AllIdentities = []string{
	ContentPublished,
	CommentCreated,
}
