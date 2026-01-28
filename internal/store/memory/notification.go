package memory

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"

	"github.com/techinsight/be-techinsights-notification-service/internal/domain"
	"github.com/techinsight/be-techinsights-notification-service/internal/store"
)

var idCounter atomic.Uint64

// NewNotificationStore returns an in-memory NotificationStore for development.
// Replace with store/mongo when MongoDB client is wired.
func NewNotificationStore() store.NotificationStore {
	return &notificationStore{
		byID:   make(map[string]*domain.Notification),
		byUser: make(map[string][]*domain.Notification),
	}
}

type notificationStore struct {
	mu     sync.RWMutex
	byID   map[string]*domain.Notification
	byUser map[string][]*domain.Notification
}

func (s *notificationStore) Create(ctx context.Context, n *domain.Notification) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if n.ID == "" {
		n.ID = genID()
	}
	cp := *n
	s.byID[n.ID] = &cp
	s.byUser[n.UserID] = append(s.byUser[n.UserID], &cp)
	return nil
}

func (s *notificationStore) GetByID(ctx context.Context, id string) (*domain.Notification, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	n, ok := s.byID[id]
	if !ok {
		return nil, nil
	}
	cp := *n
	return &cp, nil
}

func (s *notificationStore) ListByUserID(ctx context.Context, userID string, limit int) ([]*domain.Notification, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	list := s.byUser[userID]
	if limit <= 0 {
		limit = 20
	}
	if len(list) > limit {
		list = list[:limit]
	}
	out := make([]*domain.Notification, len(list))
	for i, n := range list {
		cp := *n
		out[i] = &cp
	}
	return out, nil
}

func (s *notificationStore) MarkRead(ctx context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if n, ok := s.byID[id]; ok {
		n.Read = true
		return nil
	}
	return nil
}

func genID() string {
	return fmt.Sprintf("mem-%d", idCounter.Add(1))
}
