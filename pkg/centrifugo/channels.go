package centrifugo

import "strings"

const (
	// NamespaceNoti is the Centrifugo namespace for notification channels.
	NamespaceNoti = "noti"

	// NamespaceChat is the Centrifugo namespace for chat channels.
	// Channel format: "chat:room:{roomID}"
	// Managed by the chat service — listed here for reference only.
	NamespaceChat = "chat"
)

// Channel kinds inside the noti namespace. Each has its own access rule, enforced
// in one place by gateway.ChannelPolicy.
const (
	// KindUser — "noti:user:{userID}". Private: only the owner may subscribe.
	KindUser = "user"
	// KindTopic — "noti:topic:{topic}". Public broadcast to any authenticated user.
	KindTopic = "topic"
	// KindChallenge — "noti:challenge:{challengeID}". Shared between participants;
	// requires a subscription token minted by the service that knows membership.
	KindChallenge = "challenge"
)

// NotiChannel returns the personal notification channel for a user.
// Example: "user-abc" → "noti:user:user-abc"
func NotiChannel(userID string) string {
	return NamespaceNoti + ":" + KindUser + ":" + userID
}

// ChallengeChannel returns the shared channel for a challenge's live leaderboard.
// Example: "chl-1" → "noti:challenge:chl-1"
func ChallengeChannel(challengeID string) string {
	return NamespaceNoti + ":" + KindChallenge + ":" + challengeID
}

// ParseNotiChannel splits a noti channel into its kind and id.
// Returns ok=false for channels outside the noti namespace or with an empty id.
func ParseNotiChannel(channel string) (kind, id string, ok bool) {
	rest, found := strings.CutPrefix(channel, NamespaceNoti+":")
	if !found {
		return "", "", false
	}
	kind, id, found = strings.Cut(rest, ":")
	if !found || kind == "" || id == "" {
		return "", "", false
	}
	return kind, id, true
}

// ChannelFromRoomID converts a room ID to a noti namespace channel.
// Kept for backward-compatibility with worker-dispatch.
// Example: "user:abc" → "noti:user:abc"
func ChannelFromRoomID(roomID string) string {
	return NamespaceNoti + ":" + roomID
}
