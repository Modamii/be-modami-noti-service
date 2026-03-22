package kafka

import (
	"context"
	"fmt"

	"github.com/twmb/franz-go/pkg/kgo"
	"gitlab.com/lifegoeson-libs/pkg-logging/logger"
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

	l := logger.FromContext(ctx)
	l.Info("kafka consumer started, group: " + group + ", topic: " + topic)

	for {
		fetches := client.PollFetches(ctx)
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if errs := fetches.Errors(); len(errs) > 0 {
			for _, e := range errs {
				l.Error(fmt.Sprintf("kafka fetch error: topic=%s partition=%d", e.Topic, e.Partition), e.Err)
			}
		}
		fetches.EachRecord(func(r *kgo.Record) {
			if err := handler(ctx, r.Key, r.Value); err != nil {
				l.Error(fmt.Sprintf("kafka handler error: topic=%s offset=%d", r.Topic, r.Offset), err)
			}
		})
	}
}
