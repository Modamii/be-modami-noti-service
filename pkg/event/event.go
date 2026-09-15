package event

// WSMessage is enqueued to notif:ws. Worker pops and publishes to PubSub; gateway broadcasts.
type WSMessage struct {
	RoomID  string                 `json:"room_id"`  // user:{userID} or topic:{topic}
	Event   string                 `json:"event"`
	Payload map[string]interface{} `json:"payload"`
}

// PushMessage is enqueued to notif:push. Worker pops it and calls FCM.
//
// One message per recipient: when FCM reports a token as unregistered the worker
// has to delete it, and that needs the owner.
type PushMessage struct {
	UserID        string                `json:"user_id"`
	DeviceTokens  []string              `json:"device_tokens,omitempty"`
	Subscriptions []WebPushSubscription `json:"subscriptions,omitempty"`
	Title         string                `json:"title"`
	Body          string                `json:"body"`
	Link          string                `json:"link,omitempty"`
	// EventType and Data travel with the push so tapping it can deep-link
	// straight to the right screen.
	EventType string            `json:"event_type,omitempty"`
	Data      map[string]string `json:"data,omitempty"`
}

type WebPushSubscription struct {
	Endpoint string            `json:"endpoint"`
	Keys     map[string]string `json:"keys"`
}
