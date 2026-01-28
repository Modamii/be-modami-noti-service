package kafka

import (
	"context"
	"encoding/json"
	"time"

	config "techinsights-auth-api/configs"

	"github.com/twmb/franz-go/pkg/kgo"
	"gitlab.com/services5732151/pkg-logging/logger"
)

type ProducerMessage struct {
	Key   string
	Value interface{}
}

type KafkaService struct {
	client   *kgo.Client
	appCfg   *config.Config
	getTopic func(*config.Config, string) string
}

func NewKafkaService(cfg *config.Config, getTopic func(*config.Config, string) string) (*KafkaService, error) {
	if cfg == nil || len(cfg.Kafka.Brokers) == 0 {
		return nil, nil
	}
	opts := []kgo.Opt{
		kgo.SeedBrokers(cfg.Kafka.Brokers...),
		kgo.ClientID(cfg.Kafka.ClientID),
		kgo.DialTimeout(10 * time.Second),
		kgo.ProducerBatchMaxBytes(1000000),
	}
	client, err := kgo.NewClient(opts...)
	if err != nil {
		return nil, err
	}
	return &KafkaService{
		client:   client,
		appCfg:   cfg,
		getTopic: getTopic,
	}, nil
}

func (k *KafkaService) EmitAsync(ctx context.Context, topic string, message *ProducerMessage) {
	if k == nil || k.client == nil {
		return
	}
	topicName := k.getTopic(k.appCfg, topic)
	valueBytes, err := json.Marshal(message.Value)
	if err != nil {
		logger.FromContext(ctx).Error("Failed to marshal message for Kafka", err)
		return
	}
	record := &kgo.Record{
		Topic: topicName,
		Key:   []byte(message.Key),
		Value: valueBytes,
	}
	k.client.Produce(ctx, record, func(_ *kgo.Record, err error) {
		if err != nil {
			logger.FromContext(ctx).Error("Kafka produce failed", err)
		}
	})
}
