package queue

import (
	"context"
	"encoding/json"
	"time"

	"github.com/redis/go-redis/v9"
)

// Queue uses Redis List: LPUSH to enqueue, BRPOP to consume.
type Queue struct {
	client *redis.Client
}

func New(client *redis.Client) *Queue {
	return &Queue{client: client}
}

// Enqueue pushes a JSON payload to the list.
func (q *Queue) Enqueue(ctx context.Context, key string, payload any) error {
	b, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	return q.client.LPush(ctx, key, string(b)).Err()
}

// Consume blocks on BRPOP, unmarshals into v and calls handler. Run in a loop.
func (q *Queue) Consume(ctx context.Context, key string, timeout time.Duration, handler func([]byte) error) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			result, err := q.client.BRPop(ctx, timeout, key).Result()
			if err == redis.Nil {
				// timeout — no message, loop again
				continue
			}
			if err != nil {
				return err
			}
			if len(result) < 2 {
				continue
			}
			// result[0]=key, result[1]=value
			if err := handler([]byte(result[1])); err != nil {
				return err
			}
		}
	}
}
