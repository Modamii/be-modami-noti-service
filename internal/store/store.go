package store

import (
	"context"

	"github.com/techinsight/be-techinsights-notification-service/internal/domain"
)

// NotificationStore: copy your MongoDB client into config/mongo and wire here.
type NotificationStore interface {
	Create(ctx context.Context, n *domain.Notification) error
	GetByID(ctx context.Context, id string) (*domain.Notification, error)
	ListByUserID(ctx context.Context, userID string, limit int) ([]*domain.Notification, error)
	MarkRead(ctx context.Context, id string) error
}

type SubscriberStore interface {
	Upsert(ctx context.Context, s *domain.Subscriber) error
	ByUserID(ctx context.Context, userID string) ([]*domain.Subscriber, error)
	DeleteByToken(ctx context.Context, userID, token string) error
}

type PreferenceStore interface {
	Get(ctx context.Context, userID string) (*domain.Preference, error)
	Set(ctx context.Context, p *domain.Preference) error
}
