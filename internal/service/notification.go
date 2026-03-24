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

// NotificationService orchestrates the notification pipeline:
// validate → persist → check preferences → enrich → dispatch.
type NotificationService struct {
	store           store.NotificationStore
	preferenceStore store.PreferenceStore
	subscriberStore store.SubscriberStore
	dispatchers     []ChannelDispatcher
}

func NewNotificationService(
	ns store.NotificationStore,
	ps store.PreferenceStore,
	ss store.SubscriberStore,
	dispatchers ...ChannelDispatcher,
) *NotificationService {
	return &NotificationService{
		store:           ns,
		preferenceStore: ps,
		subscriberStore: ss,
		dispatchers:     dispatchers,
	}
}

// Process runs the full notification pipeline for the given params.
func (svc *NotificationService) Process(ctx context.Context, params *NotificationParams) error {
	l := logger.FromContext(ctx)

	// 1. Persist notifications
	svc.persistNotifications(ctx, params)

	// 2. Determine which channels this identity should dispatch to
	channels := contract.IdentityChannels[params.Identity]

	// 3. For each channel, check per-user preferences and dispatch
	for _, ch := range channels {
		filteredParams := svc.filterByPreference(ctx, params, ch)
		if len(filteredParams.UserIDs) == 0 {
			l.Debug("no recipients after preference filtering for channel: " + ch)
			continue
		}

		// 4. Enrich params if needed (e.g. resolve device tokens for push)
		enriched := svc.enrich(ctx, filteredParams, ch)

		// 5. Dispatch via matching strategy
		for _, d := range svc.dispatchers {
			if d.Channel() == ch {
				if err := d.Dispatch(ctx, enriched); err != nil {
					l.Error("dispatch failed for channel: "+ch, err)
					return err
				}
			}
		}
	}
	return nil
}

// filterByPreference removes users who have disabled the given channel.
func (svc *NotificationService) filterByPreference(ctx context.Context, params *NotificationParams, channel string) *NotificationParams {
	if svc.preferenceStore == nil {
		return params
	}

	filtered := make([]string, 0, len(params.UserIDs))
	for _, uid := range params.UserIDs {
		pref, err := svc.preferenceStore.Get(ctx, uid)
		if err != nil {
			// On error, default to allowing delivery
			logger.FromContext(ctx).Error("failed to get preference for user: "+uid, err)
			filtered = append(filtered, uid)
			continue
		}
		if isChannelEnabled(pref, channel) {
			filtered = append(filtered, uid)
		}
	}

	return &NotificationParams{
		Identity: params.Identity,
		Title:    params.Title,
		Body:     params.Body,
		Link:     params.Link,
		Extra:    params.Extra,
		UserIDs:  filtered,
	}
}

func isChannelEnabled(pref *domain.Preference, channel string) bool {
	switch channel {
	case contract.ChannelInApp:
		return pref.InAppEnabled
	case contract.ChannelPush:
		return pref.PushEnabled
	default:
		return true
	}
}

// enrich adds channel-specific data to params (e.g. device tokens for push).
func (svc *NotificationService) enrich(ctx context.Context, params *NotificationParams, channel string) *NotificationParams {
	if channel != contract.ChannelPush || svc.subscriberStore == nil {
		return params
	}

	// Resolve device tokens for push recipients
	var tokens []string
	for _, uid := range params.UserIDs {
		subs, err := svc.subscriberStore.ByUserID(ctx, uid)
		if err != nil {
			logger.FromContext(ctx).Error("failed to get subscribers for user: "+uid, err)
			continue
		}
		for _, s := range subs {
			if s.DeviceToken != "" {
				tokens = append(tokens, s.DeviceToken)
			}
		}
	}

	enrichedExtra := make(map[string]interface{}, len(params.Extra)+1)
	for k, v := range params.Extra {
		enrichedExtra[k] = v
	}
	enrichedExtra["device_tokens"] = tokens

	return &NotificationParams{
		Identity: params.Identity,
		Title:    params.Title,
		Body:     params.Body,
		Link:     params.Link,
		Extra:    enrichedExtra,
		UserIDs:  params.UserIDs,
	}
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
