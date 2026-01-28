package domain

type Preference struct {
	UserID       string `bson:"user_id" json:"user_id"`
	InAppEnabled bool   `bson:"in_app_enabled" json:"in_app_enabled"`
	PushEnabled  bool   `bson:"push_enabled" json:"push_enabled"`
}
