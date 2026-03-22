package database

import (
	"context"
	"log/slog"
	"time"

	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/readpref"
)

type MongoConfig struct {
	URI           string
	Database      string
	Timeout       time.Duration
	MaxPool       uint64
	MinPool       uint64
	EnableLogging bool
}

type MongoDB struct {
	Client   *mongo.Client
	Database *mongo.Database
}

type MongoLogSink struct{}

func (m *MongoLogSink) Info(level int, message string, keysAndValues ...interface{}) {}
func (m *MongoLogSink) Error(err error, message string, keysAndValues ...interface{}) {}

func NewMongoDB(config MongoConfig) (*MongoDB, error) {
	ctx := context.Background()

	if config.Timeout == 0 {
		config.Timeout = 30 * time.Second
	}
	if config.MaxPool == 0 {
		config.MaxPool = 100
	}
	if config.MinPool == 0 {
		config.MinPool = 10
	}

	var loggerOpts *options.LoggerOptions
	if config.EnableLogging {
		loggerOpts = &options.LoggerOptions{
			ComponentLevels: map[options.LogComponent]options.LogLevel{},
			Sink:            &MongoLogSink{},
		}
	} else {
		loggerOpts = &options.LoggerOptions{
			ComponentLevels: map[options.LogComponent]options.LogLevel{},
		}
	}

	clientOpts := options.Client().
		ApplyURI(config.URI).
		SetMaxPoolSize(config.MaxPool).
		SetMinPoolSize(config.MinPool).
		SetMaxConnIdleTime(5 * time.Minute).
		SetServerSelectionTimeout(config.Timeout).
		SetLoggerOptions(loggerOpts)

	connectCtx, cancel := context.WithTimeout(ctx, config.Timeout)
	defer cancel()

	client, err := mongo.Connect(connectCtx, clientOpts)
	if err != nil {
		slog.Error("failed to connect to MongoDB", "error", err)
		return nil, err
	}
	if err := client.Ping(connectCtx, readpref.Primary()); err != nil {
		slog.Error("failed to ping MongoDB", "error", err)
		return nil, err
	}
	slog.Info("connected to MongoDB")
	return &MongoDB{
		Client:   client,
		Database: client.Database(config.Database),
	}, nil
}

func (m *MongoDB) Close(ctx context.Context) error {
	if err := m.Client.Disconnect(ctx); err != nil {
		slog.Error("failed to disconnect from MongoDB", "error", err)
		return err
	}
	slog.Info("disconnected from MongoDB")
	return nil
}

func (m *MongoDB) Ping(ctx context.Context) error {
	return m.Client.Ping(ctx, readpref.Primary())
}

func (m *MongoDB) GetCollection(name string) *mongo.Collection {
	return m.Database.Collection(name)
}
