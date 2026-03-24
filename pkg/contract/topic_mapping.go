package contract

// TopicToIdentity maps Kafka topic base names to notification identities.
// This bridges domain events (e.g. "techinsight.article.created") to
// the notification contract identity (e.g. "content_published").
var TopicToIdentity = map[string]string{
	"techinsight.article.created": ContentPublished,
	"techinsight.comment.created": CommentCreated,
}

// IdentityFromTopic returns the notification identity for a given Kafka topic base name.
// Returns empty string if the topic does not trigger notifications.
func IdentityFromTopic(baseTopic string) (string, bool) {
	id, ok := TopicToIdentity[baseTopic]
	return id, ok
}
