package database

import (
	"context"
	"time"

	"gitlab.com/lifegoeson-libs/pkg-logging/logger"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/readpref"
)

// MongoConfig holds MongoDB connection settings.
type MongoConfig struct {
	URI      string
	Database string
	Timeout  time.Duration
	MaxPool  uint64
	MinPool  uint64
}

// MongoDB wraps a mongo client and database.
type MongoDB struct {
	Client   *mongo.Client
	Database *mongo.Database
}

// NewMongoDB creates a new MongoDB connection.
func NewMongoDB(config MongoConfig) (*MongoDB, error) {
	ctx := context.Background()
	l := logger.FromContext(ctx)

	if config.Timeout == 0 {
		config.Timeout = 30 * time.Second
	}
	if config.MaxPool == 0 {
		config.MaxPool = 100
	}
	if config.MinPool == 0 {
		config.MinPool = 10
	}

	clientOpts := options.Client().
		ApplyURI(config.URI).
		SetMaxPoolSize(config.MaxPool).
		SetMinPoolSize(config.MinPool).
		SetMaxConnIdleTime(5 * time.Minute).
		SetServerSelectionTimeout(config.Timeout)

	connectCtx, cancel := context.WithTimeout(ctx, config.Timeout)
	defer cancel()

	client, err := mongo.Connect(connectCtx, clientOpts)
	if err != nil {
		l.Error("failed to connect to MongoDB", err)
		return nil, err
	}

	if err := client.Ping(connectCtx, readpref.Primary()); err != nil {
		l.Error("failed to ping MongoDB", err)
		return nil, err
	}

	l.Info("connected to MongoDB")
	return &MongoDB{
		Client:   client,
		Database: client.Database(config.Database),
	}, nil
}

// Close disconnects the MongoDB client.
func (m *MongoDB) Close(ctx context.Context) error {
	return m.Client.Disconnect(ctx)
}
