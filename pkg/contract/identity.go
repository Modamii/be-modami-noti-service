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

// IsValidIdentity checks whether the given identity is registered.
func IsValidIdentity(identity string) bool {
	for _, id := range AllIdentities {
		if id == identity {
			return true
		}
	}
	return false
}
