package domain

type Subscriber struct {
	ID           string `bson:"_id,omitempty" json:"id"`
	UserID       string `bson:"user_id" json:"user_id"`
	DeviceToken  string `bson:"device_token" json:"device_token"`
	Platform     string `bson:"platform" json:"platform"` // ios, android, web
	WebPushEndpoint string `bson:"web_push_endpoint,omitempty" json:"web_push_endpoint,omitempty"`
	WebPushKeys  map[string]string `bson:"web_push_keys,omitempty" json:"web_push_keys,omitempty"`
}
