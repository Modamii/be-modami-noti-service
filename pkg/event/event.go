package event

// WSMessage is enqueued to notif:ws. Worker pops and publishes to PubSub; gateway broadcasts.
type WSMessage struct {
	RoomID  string                 `json:"room_id"`  // user:{userID} or topic:{topic}
	Event   string                 `json:"event"`
	Payload map[string]interface{} `json:"payload"`
}

// PushMessage is enqueued to notif:push. Worker pops and calls FCM/Web Push (stub logs).
type PushMessage struct {
	DeviceTokens []string               `json:"device_tokens,omitempty"`
	Subscriptions []WebPushSubscription `json:"subscriptions,omitempty"`
	Title        string                 `json:"title"`
	Body         string                 `json:"body"`
	Link         string                 `json:"link,omitempty"`
}

type WebPushSubscription struct {
	Endpoint string            `json:"endpoint"`
	Keys     map[string]string `json:"keys"`
}
