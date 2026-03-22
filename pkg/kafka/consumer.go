package kafka

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/twmb/franz-go/pkg/kgo"
)

// RunConsumer starts a Kafka consumer that reads from the given topic
// and calls handler for each record value. Blocks until ctx is cancelled.
func RunConsumer(ctx context.Context, brokers []string, group, topic string, handler func(ctx context.Context, key, value []byte) error) error {
	client, err := kgo.NewClient(
		kgo.SeedBrokers(brokers...),
		kgo.ConsumerGroup(group),
		kgo.ConsumeTopics(topic),
		kgo.ConsumeResetOffset(kgo.NewOffset().AtStart()),
	)
	if err != nil {
		return fmt.Errorf("kafka: create client: %w", err)
	}
	defer client.Close()

	slog.Info("kafka consumer started", "group", group, "topic", topic)

	for {
		fetches := client.PollFetches(ctx)
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if errs := fetches.Errors(); len(errs) > 0 {
			for _, e := range errs {
				slog.Error("kafka fetch error", "topic", e.Topic, "partition", e.Partition, "error", e.Err)
			}
		}
		fetches.EachRecord(func(r *kgo.Record) {
			if err := handler(ctx, r.Key, r.Value); err != nil {
				slog.Error("kafka handler error", "topic", r.Topic, "offset", r.Offset, "error", err)
			}
		})
	}
}
