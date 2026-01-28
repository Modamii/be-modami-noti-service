package queue

import (
	"context"
	"encoding/json"
	"log"
)

// PubSub: worker-ws publishes to channel; ws-gateway subscribes and broadcasts.
// Your Redis connection is in config/redis (copy your client there). This package uses the same client.

// PublishWSDispatch publishes a WS message to the channel so ws-gateway can broadcast.
func (q *Queue) PublishWSDispatch(ctx context.Context, channel string, payload interface{}) error {
	b, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	return q.client.Publish(ctx, channel, string(b)).Err()
}

// SubscribeWSDispatch subscribes to the channel and calls handler for each message.
func (q *Queue) SubscribeWSDispatch(ctx context.Context, channel string, handler func([]byte) error) error {
	pubsub := q.client.Subscribe(ctx, channel)
	defer pubsub.Close()
	msgCh := pubsub.Channel()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case msg, ok := <-msgCh:
			if !ok {
				return nil
			}
			if err := handler([]byte(msg.Payload)); err != nil {
				log.Printf("SubscribeWSDispatch handler error: %v", err)
			}
		}
	}
}
