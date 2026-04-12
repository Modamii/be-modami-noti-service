package centrifugo

const (
	// NamespaceNoti is the Centrifugo namespace for notification channels.
	// Channel format: "noti:user:{userID}"
	NamespaceNoti = "noti"

	// NamespaceChat is the Centrifugo namespace for chat channels.
	// Channel format: "chat:room:{roomID}"
	// Managed by the chat service — listed here for reference only.
	NamespaceChat = "chat"
)

// NotiChannel returns the personal notification channel for a user.
// Example: "user-abc" → "noti:user:user-abc"
func NotiChannel(userID string) string {
	return NamespaceNoti + ":user:" + userID
}

// ChannelFromRoomID converts a room ID to a noti namespace channel.
// Kept for backward-compatibility with worker-dispatch.
// Example: "user:abc" → "noti:user:abc"
func ChannelFromRoomID(roomID string) string {
	return NamespaceNoti + ":" + roomID
}
