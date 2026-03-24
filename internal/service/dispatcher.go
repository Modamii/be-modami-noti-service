package service

import (
	"context"

	"github.com/techinsight/be-techinsights-notification-service/internal/queue"
	"github.com/techinsight/be-techinsights-notification-service/pkg/contract"
	"github.com/techinsight/be-techinsights-notification-service/pkg/event"
)

// InAppDispatcher implements ChannelDispatcher for WebSocket (in-app) delivery.
type InAppDispatcher struct {
	queue    *queue.Queue
	queueKey string
}

func NewInAppDispatcher(q *queue.Queue, queueKey string) *InAppDispatcher {
	return &InAppDispatcher{queue: q, queueKey: queueKey}
}

func (d *InAppDispatcher) Channel() string { return contract.ChannelInApp }

func (d *InAppDispatcher) Dispatch(ctx context.Context, params *NotificationParams) error {
	for _, uid := range params.UserIDs {
		roomID := "user:" + uid
		msg := event.WSMessage{
			RoomID:  roomID,
			Event:   params.Identity,
			Payload: params.Extra,
		}
		if err := d.queue.Enqueue(ctx, d.queueKey, msg); err != nil {
			return err
		}
	}
	return nil
}

// PushDispatcher implements ChannelDispatcher for push notification delivery.
type PushDispatcher struct {
	queue    *queue.Queue
	queueKey string
}

func NewPushDispatcher(q *queue.Queue, queueKey string) *PushDispatcher {
	return &PushDispatcher{queue: q, queueKey: queueKey}
}

func (d *PushDispatcher) Channel() string { return contract.ChannelPush }

func (d *PushDispatcher) Dispatch(ctx context.Context, params *NotificationParams) error {
	msg := event.PushMessage{
		Title: params.Title,
		Body:  params.Body,
		Link:  params.Link,
	}
	return d.queue.Enqueue(ctx, d.queueKey, msg)
}
