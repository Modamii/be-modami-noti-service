package contract

// Channel names for delivery (in_app → ws queue, push → push queue, email → email queue).
const (
	ChannelInApp = "in_app"
	ChannelPush  = "push"
	ChannelEmail = "email"
)

// IdentityChannels maps identity → channels to use. Handler uses this to decide which queues to enqueue.
var IdentityChannels = map[string][]string{
	ContentPublished: {ChannelInApp, ChannelPush},
	CommentCreated:   {ChannelInApp, ChannelPush},
}
