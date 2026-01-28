package domain

import "time"

type Notification struct {
	ID         string                 `bson:"_id,omitempty" json:"id"`
	UserID     string                 `bson:"user_id" json:"user_id"`
	EventType  string                 `bson:"event_type" json:"event_type"`
	Title      string                 `bson:"title" json:"title"`
	Body       string                 `bson:"body" json:"body"`
	Link       string                 `bson:"link,omitempty" json:"link,omitempty"`
	Read       bool                   `bson:"read" json:"read"`
	Extra      map[string]interface{} `bson:"extra,omitempty" json:"extra,omitempty"`
	CreatedAt  time.Time              `bson:"created_at" json:"created_at"`
}
