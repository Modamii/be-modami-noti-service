package store

import (
	"context"

	"be-modami-no-service/internal/domain"
)

// PaginationParams for paginated list queries.
type PaginationParams struct {
	Page       int
	PerPage    int
	UnreadOnly bool
}

// PaginatedResult holds paginated query results.
type PaginatedResult struct {
	Items      []*domain.Notification
	Total      int64
	Page       int
	PerPage    int
	TotalPages int
	HasMore    bool
}

// NotificationStore: MongoDB-backed notification storage.
//
// Every single-document method takes userID and scopes the query by it, so a
// caller can never read or mutate a notification belonging to someone else.
type NotificationStore interface {
	Create(ctx context.Context, n *domain.Notification) error
	GetByID(ctx context.Context, userID, id string) (*domain.Notification, error)
	ListByUserID(ctx context.Context, userID string, limit int) ([]*domain.Notification, error)
	ListByUserIDPaginated(ctx context.Context, userID string, params PaginationParams) (*PaginatedResult, error)
	// MarkRead reports whether a notification owned by userID was updated.
	MarkRead(ctx context.Context, userID, id string) (bool, error)
	MarkAllRead(ctx context.Context, userID string) (int64, error)
	// Delete reports whether a notification owned by userID was removed.
	Delete(ctx context.Context, userID, id string) (bool, error)
	CountUnread(ctx context.Context, userID string) (int64, error)
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
