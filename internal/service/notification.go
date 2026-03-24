package service

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/techinsight/be-techinsights-notification-service/internal/domain"
	"github.com/techinsight/be-techinsights-notification-service/internal/store"
	"github.com/techinsight/be-techinsights-notification-service/pkg/contract"
	"gitlab.com/lifegoeson-libs/pkg-logging/logger"
)

// NotificationParams holds event-specific data extracted by a handler.
type NotificationParams struct {
	Identity string
	Title    string
	Body     string
	Link     string
	Extra    map[string]interface{}
	UserIDs  []string
}

// ChannelDispatcher defines a strategy for dispatching notifications to a delivery channel.
type ChannelDispatcher interface {
	Channel() string
	Dispatch(ctx context.Context, params *NotificationParams) error
}

// NotificationService orchestrates notification creation and multi-channel dispatch.
type NotificationService struct {
	store       store.NotificationStore
	dispatchers []ChannelDispatcher
}

func NewNotificationService(s store.NotificationStore, dispatchers ...ChannelDispatcher) *NotificationService {
	return &NotificationService{
		store:       s,
		dispatchers: dispatchers,
	}
}

// Process persists notifications for each recipient, then dispatches to configured channels.
func (svc *NotificationService) Process(ctx context.Context, params *NotificationParams) error {
	svc.persistNotifications(ctx, params)

	channels := contract.IdentityChannels[params.Identity]
	for _, ch := range channels {
		for _, d := range svc.dispatchers {
			if d.Channel() == ch {
				if err := d.Dispatch(ctx, params); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func (svc *NotificationService) persistNotifications(ctx context.Context, params *NotificationParams) {
	for _, uid := range params.UserIDs {
		notif := &domain.Notification{
			ID:        uuid.New().String(),
			UserID:    uid,
			EventType: params.Identity,
			Title:     params.Title,
			Body:      params.Body,
			Link:      params.Link,
			Read:      false,
			Extra:     params.Extra,
			CreatedAt: time.Now(),
		}
		if err := svc.store.Create(ctx, notif); err != nil {
			logger.FromContext(ctx).Error("failed to persist notification", err)
		}
	}
}
