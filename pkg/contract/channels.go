package contract

// Channel names for delivery (in_app → ws queue, push → push queue, email → email queue).
const (
	ChannelInApp = "in_app"
	ChannelPush  = "push"
	ChannelEmail = "email"
)

// IdentityChannels maps identity → channels to use. The service uses this to
// decide which queues to enqueue, after per-user preference filtering.
var IdentityChannels = map[string][]string{
	ContentPublished: {ChannelInApp, ChannelPush},
	CommentCreated:   {ChannelInApp, ChannelPush},

	VideoReady:          {ChannelInApp, ChannelPush},
	VideoFailed:         {ChannelInApp, ChannelPush},
	FlashcardDue:        {ChannelInApp, ChannelPush},
	AchievementUnlocked: {ChannelInApp, ChannelPush},
	ChallengeEnded:      {ChannelInApp, ChannelPush},

	// Progress ticks are only useful while the user is watching the screen —
	// pushing them would be noise.
	VideoProgress:        {ChannelInApp},
	ChallengeLeaderboard: {ChannelInApp},

	// Only meaningful when the app is closed; in-app would be redundant with the
	// streak counter already on screen.
	StreakAtRisk: {ChannelPush},
}

// TransientIdentities are live status updates, not things to read later: a video
// pipeline tick or a leaderboard change is stale within seconds and meaningless
// once the screen is closed.
//
// They are delivered over WebSocket but never stored, so six pipeline steps do
// not leave six rows in the user's notification list.
var TransientIdentities = map[string]bool{
	VideoProgress:        true,
	ChallengeLeaderboard: true,
}

// IsTransient reports whether an identity should be delivered without being
// persisted.
func IsTransient(identity string) bool {
	return TransientIdentities[identity]
}

// Client-sync events are published on a user's personal channel but are not
// notifications: they tell other devices that state changed here, so a badge
// read on the phone clears on the tablet too. They are never persisted and have
// no entry in IdentityChannels.
const (
	// EventNotificationRead — one notification was marked read.
	EventNotificationRead = "notification_read"
	// EventNotificationReadAll — every notification was marked read.
	EventNotificationReadAll = "notification_read_all"
)
