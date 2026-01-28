package events

type UserCreatedEvent struct {
	EventType  string `json:"eventType"`
	UserID     string `json:"userId"`
	Email      string `json:"email"`
	Username   string `json:"username"`
	FirstName  string `json:"firstName"`
	LastName   string `json:"lastName"`
	Image      string `json:"image"`
	Role       string `json:"role"`
	Status     string `json:"status"`
}

func NewUserCreatedEvent(userID, email, username, firstName, lastName, image, role, status string) *UserCreatedEvent {
	return &UserCreatedEvent{
		EventType: "user_created",
		UserID:    userID,
		Email:     email,
		Username:  username,
		FirstName: firstName,
		LastName:  lastName,
		Image:     image,
		Role:      role,
		Status:    status,
	}
}
