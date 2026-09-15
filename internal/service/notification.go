package service

import (
	"context"
	"time"

	"be-modami-no-service/internal/domain"
	"be-modami-no-service/internal/store"
	"be-modami-no-service/pkg/contract"

	"github.com/google/uuid"
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

	// Channel, when set, is a challenge ID: the event is broadcast once to the
	// shared channel instead of to each recipient's personal channel.
	Channel string

	// Filled in by Process — handlers leave these zero.
	Persisted    map[string]*domain.Notification // userID → the stored notification
	UnreadCount  map[string]int64                // userID → unread total after persisting
	DeviceTokens map[string][]string             // userID → push tokens, set during enrich
}

// withRecipients returns a shallow copy carrying a different recipient list.
// Copying the struct rather than rebuilding it field by field means a new field
// can never be silently dropped from the pipeline.
func (p *NotificationParams) withRecipients(userIDs []string) *NotificationParams {
	cp := *p
	cp.UserIDs = userIDs
	return &cp
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

	// 1. Persist notifications, and remember them so dispatchers can send a payload
	// the client can render without a follow-up API call. Transient identities
	// skip this: they are live status, not history, and counting them as unread
	// would make the bell badge meaningless.
	if !contract.IsTransient(params.Identity) {
		params.Persisted = svc.persistNotifications(ctx, params)
		params.UnreadCount = svc.unreadCounts(ctx, params.UserIDs)
	}

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

// filterByPreference removes users who should not receive this notification on
// this channel. A preference lookup that fails defaults to delivering: a missed
// notification is worse than one the user could have muted.
func (svc *NotificationService) filterByPreference(ctx context.Context, params *NotificationParams, channel string) *NotificationParams {
	if svc.preferenceStore == nil {
		return params
	}

	now := time.Now()
	filtered := make([]string, 0, len(params.UserIDs))
	for _, uid := range params.UserIDs {
		pref, err := svc.preferenceStore.Get(ctx, uid)
		if err != nil {
			logger.FromContext(ctx).Error("failed to get preference for user: "+uid, err)
			filtered = append(filtered, uid)
			continue
		}
		if allowed(pref, params.Identity, channel, now) {
			filtered = append(filtered, uid)
		}
	}

	return params.withRecipients(filtered)
}

// allowed applies the three preference gates in order: the channel switch, the
// per-type mute, then quiet hours.
func allowed(pref *domain.Preference, identity, channel string, now time.Time) bool {
	if pref.IsMuted(identity) {
		return false
	}
	switch channel {
	case contract.ChannelInApp:
		return pref.InAppEnabled
	case contract.ChannelPush:
		// Quiet hours apply to push only — in-app is silent and the user is
		// already looking at the screen.
		return pref.PushEnabled && !pref.InQuietHours(now)
	default:
		return true
	}
}

// enrich adds channel-specific data to params (device tokens for push).
//
// Tokens stay grouped by user rather than flattened into one list: the push
// worker deletes tokens FCM rejects, and that requires knowing the owner.
func (svc *NotificationService) enrich(ctx context.Context, params *NotificationParams, channel string) *NotificationParams {
	if channel != contract.ChannelPush || svc.subscriberStore == nil {
		return params
	}

	tokens := make(map[string][]string, len(params.UserIDs))
	for _, uid := range params.UserIDs {
		subs, err := svc.subscriberStore.ByUserID(ctx, uid)
		if err != nil {
			logger.FromContext(ctx).Error("failed to get subscribers for user: "+uid, err)
			continue
		}
		for _, s := range subs {
			if s.DeviceToken != "" {
				tokens[uid] = append(tokens[uid], s.DeviceToken)
			}
		}
	}

	cp := *params
	cp.DeviceTokens = tokens
	return &cp
}

func (svc *NotificationService) persistNotifications(ctx context.Context, params *NotificationParams) map[string]*domain.Notification {
	stored := make(map[string]*domain.Notification, len(params.UserIDs))
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
			continue
		}
		stored[uid] = notif
	}
	return stored
}

// unreadCounts reads each recipient's unread total so the in-app payload can
// carry a badge number the client applies without another round trip.
func (svc *NotificationService) unreadCounts(ctx context.Context, userIDs []string) map[string]int64 {
	counts := make(map[string]int64, len(userIDs))
	for _, uid := range userIDs {
		n, err := svc.store.CountUnread(ctx, uid)
		if err != nil {
			logger.FromContext(ctx).Error("failed to count unread for user: "+uid, err)
			continue
		}
		counts[uid] = n
	}
	return counts
}
