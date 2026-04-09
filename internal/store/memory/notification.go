package memory

import (
	"context"
	"fmt"
	"math"
	"sync"
	"sync/atomic"

	"be-modami-no-service/internal/domain"
	"be-modami-no-service/internal/store"
)

var idCounter atomic.Uint64

// NewNotificationStore returns an in-memory NotificationStore for development.
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

func (s *notificationStore) Create(_ context.Context, n *domain.Notification) error {
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

func (s *notificationStore) GetByID(_ context.Context, id string) (*domain.Notification, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	n, ok := s.byID[id]
	if !ok {
		return nil, nil
	}
	cp := *n
	return &cp, nil
}

func (s *notificationStore) ListByUserID(_ context.Context, userID string, limit int) ([]*domain.Notification, error) {
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

func (s *notificationStore) ListByUserIDPaginated(_ context.Context, userID string, params store.PaginationParams) (*store.PaginatedResult, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var filtered []*domain.Notification
	for _, n := range s.byUser[userID] {
		if params.UnreadOnly && n.Read {
			continue
		}
		filtered = append(filtered, n)
	}

	total := int64(len(filtered))
	page := params.Page
	if page <= 0 {
		page = 1
	}
	perPage := params.PerPage
	if perPage <= 0 {
		perPage = 20
	}

	start := (page - 1) * perPage
	if start >= len(filtered) {
		return &store.PaginatedResult{
			Items:      []*domain.Notification{},
			Total:      total,
			Page:       page,
			PerPage:    perPage,
			TotalPages: int(math.Ceil(float64(total) / float64(perPage))),
		}, nil
	}
	end := start + perPage
	if end > len(filtered) {
		end = len(filtered)
	}

	out := make([]*domain.Notification, end-start)
	for i, n := range filtered[start:end] {
		cp := *n
		out[i] = &cp
	}

	totalPages := int(math.Ceil(float64(total) / float64(perPage)))
	return &store.PaginatedResult{
		Items:      out,
		Total:      total,
		Page:       page,
		PerPage:    perPage,
		TotalPages: totalPages,
		HasMore:    page < totalPages,
	}, nil
}

func (s *notificationStore) MarkRead(_ context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if n, ok := s.byID[id]; ok {
		n.Read = true
	}
	return nil
}

func (s *notificationStore) MarkAllRead(_ context.Context, userID string) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var count int64
	for _, n := range s.byUser[userID] {
		if !n.Read {
			n.Read = true
			count++
		}
	}
	return count, nil
}

func (s *notificationStore) Delete(_ context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	n, ok := s.byID[id]
	if !ok {
		return nil
	}
	delete(s.byID, id)
	list := s.byUser[n.UserID]
	for i, item := range list {
		if item.ID == id {
			s.byUser[n.UserID] = append(list[:i], list[i+1:]...)
			break
		}
	}
	return nil
}

func (s *notificationStore) CountUnread(_ context.Context, userID string) (int64, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var count int64
	for _, n := range s.byUser[userID] {
		if !n.Read {
			count++
		}
	}
	return count, nil
}

func genID() string {
	return fmt.Sprintf("mem-%d", idCounter.Add(1))
}
