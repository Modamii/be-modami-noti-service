package contract

// Identity constants (string values used in Kafka message value).
// Producers and notification service share this set.
const (
	// techinsight / modami
	ContentPublished = "content_published"
	CommentCreated   = "comment_created"

	// lingocast — the producer supplies title, body and link in do[0].data,
	// so these all share one generic handler (see internal/handlers/lingocast.go).
	VideoReady           = "video_ready"
	VideoFailed          = "video_failed"
	VideoProgress        = "video_progress"
	FlashcardDue         = "flashcard_due"
	StreakAtRisk         = "streak_at_risk"
	AchievementUnlocked  = "achievement_unlocked"
	ChallengeLeaderboard = "challenge_leaderboard"
	ChallengeEnded       = "challenge_ended"
)

// LingocastIdentities are the identities whose copy is owned by the producing
// service rather than by a bespoke handler here.
var LingocastIdentities = []string{
	VideoReady,
	VideoFailed,
	VideoProgress,
	FlashcardDue,
	StreakAtRisk,
	AchievementUnlocked,
	ChallengeLeaderboard,
	ChallengeEnded,
}

// AllIdentities lists every identity for validation/documentation.
var AllIdentities = append([]string{
	ContentPublished,
	CommentCreated,
}, LingocastIdentities...)

// IsValidIdentity checks whether the given identity is registered.
func IsValidIdentity(identity string) bool {
	for _, id := range AllIdentities {
		if id == identity {
			return true
		}
	}
	return false
}
