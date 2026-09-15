package api

import (
	"context"

	"be-modami-no-service/internal/queue"
	"be-modami-no-service/pkg/centrifugo"
	"be-modami-no-service/pkg/contract"
	"be-modami-no-service/pkg/event"
)

// ReadSyncPublisher tells a user's other devices that they read something here.
// Without it the bell badge drifts: clearing a notification on the phone leaves
// it unread on the tablet until that device refetches.
type ReadSyncPublisher interface {
	NotificationRead(ctx context.Context, userID, notificationID string, unread int64)
	NotificationReadAll(ctx context.Context, userID string)
}

type queueReadSync struct {
	queue    *queue.Queue
	queueKey string
}

// NewReadSyncPublisher publishes through the same Redis queue the dispatcher
// uses, so read events travel the same path as notifications.
func NewReadSyncPublisher(q *queue.Queue, queueKey string) ReadSyncPublisher {
	return &queueReadSync{queue: q, queueKey: queueKey}
}

func (p *queueReadSync) NotificationRead(ctx context.Context, userID, notificationID string, unread int64) {
	p.publish(ctx, userID, event.WSMessage{
		RoomID: centrifugo.KindUser + ":" + userID,
		Event:  contract.EventNotificationRead,
		Payload: map[string]interface{}{
			"id":           notificationID,
			"unread_count": unread,
		},
	})
}

func (p *queueReadSync) NotificationReadAll(ctx context.Context, userID string) {
	p.publish(ctx, userID, event.WSMessage{
		RoomID: centrifugo.KindUser + ":" + userID,
		Event:  contract.EventNotificationReadAll,
		Payload: map[string]interface{}{
			"unread_count": int64(0),
		},
	})
}

// publish is best-effort: the read has already been persisted, and a failed sync
// only means another device catches up on its next refetch.
func (p *queueReadSync) publish(ctx context.Context, _ string, msg event.WSMessage) {
	_ = p.queue.Enqueue(ctx, p.queueKey, msg)
}
