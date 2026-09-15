package service

import (
	"context"

	"be-modami-no-service/internal/queue"
	"be-modami-no-service/pkg/centrifugo"
	"be-modami-no-service/pkg/contract"
	"be-modami-no-service/pkg/event"
)

// InAppDispatcher implements ChannelDispatcher for WebSocket (in-app) delivery via Centrifugo.
type InAppDispatcher struct {
	queue    *queue.Queue
	queueKey string
}

func NewInAppDispatcher(q *queue.Queue, queueKey string) *InAppDispatcher {
	return &InAppDispatcher{queue: q, queueKey: queueKey}
}

func (d *InAppDispatcher) Channel() string { return contract.ChannelInApp }

// Dispatch enqueues one WebSocket message per recipient, carrying everything the
// client needs to render the notification immediately — no follow-up API call.
//
// When params.Channel is set the event is a broadcast: it goes out once to the
// shared channel instead of to each recipient individually.
func (d *InAppDispatcher) Dispatch(ctx context.Context, params *NotificationParams) error {
	if params.Channel != "" {
		return d.queue.Enqueue(ctx, d.queueKey, event.WSMessage{
			RoomID:  centrifugo.KindChallenge + ":" + params.Channel,
			Event:   params.Identity,
			Payload: basePayload(params),
		})
	}

	for _, uid := range params.UserIDs {
		payload := basePayload(params)
		if n := params.Persisted[uid]; n != nil {
			payload["id"] = n.ID
			payload["created_at"] = n.CreatedAt
		}
		if count, ok := params.UnreadCount[uid]; ok {
			payload["unread_count"] = count
		}

		msg := event.WSMessage{
			RoomID:  centrifugo.KindUser + ":" + uid,
			Event:   params.Identity,
			Payload: payload,
		}
		if err := d.queue.Enqueue(ctx, d.queueKey, msg); err != nil {
			return err
		}
	}
	return nil
}

// basePayload builds the fields shared by every recipient. Extra is copied rather
// than referenced so per-recipient additions never leak across messages.
func basePayload(params *NotificationParams) map[string]interface{} {
	payload := make(map[string]interface{}, len(params.Extra)+6)
	for k, v := range params.Extra {
		payload[k] = v
	}
	// device_tokens is a push-delivery detail; it must never reach a client.
	delete(payload, "device_tokens")

	payload["event_type"] = params.Identity
	payload["title"] = params.Title
	payload["body"] = params.Body
	if params.Link != "" {
		payload["link"] = params.Link
	}
	return payload
}

// PushDispatcher implements ChannelDispatcher for push notification delivery (FCM/Web Push).
type PushDispatcher struct {
	queue    *queue.Queue
	queueKey string
}

func NewPushDispatcher(q *queue.Queue, queueKey string) *PushDispatcher {
	return &PushDispatcher{queue: q, queueKey: queueKey}
}

func (d *PushDispatcher) Channel() string { return contract.ChannelPush }

// Dispatch enqueues one push job per recipient who has at least one device.
func (d *PushDispatcher) Dispatch(ctx context.Context, params *NotificationParams) error {
	for _, uid := range params.UserIDs {
		tokens := params.DeviceTokens[uid]
		if len(tokens) == 0 {
			continue // no registered device — nothing to enqueue
		}
		msg := event.PushMessage{
			UserID:       uid,
			DeviceTokens: tokens,
			Title:        params.Title,
			Body:         params.Body,
			Link:         params.Link,
			EventType:    params.Identity,
			Data:         pushData(params),
		}
		if err := d.queue.Enqueue(ctx, d.queueKey, msg); err != nil {
			return err
		}
	}
	return nil
}

// pushData is the key/value bundle FCM delivers alongside the alert. FCM only
// accepts string values, so anything else is dropped rather than coerced into
// something the client would have to guess at.
func pushData(params *NotificationParams) map[string]string {
	data := make(map[string]string, len(params.Extra)+2)
	for k, v := range params.Extra {
		if s, ok := v.(string); ok {
			data[k] = s
		}
	}
	data["event_type"] = params.Identity
	if params.Link != "" {
		data["link"] = params.Link
	}
	return data
}
