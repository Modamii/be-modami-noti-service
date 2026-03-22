package mongo

import (
	"context"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// EnsureIndexes creates required indexes for all collections.
func EnsureIndexes(ctx context.Context, db *mongo.Database) error {
	// notifications: compound index for user queries sorted by time
	_, err := db.Collection(notificationsCollection).Indexes().CreateMany(ctx, []mongo.IndexModel{
		{
			Keys:    bson.D{{Key: "user_id", Value: 1}, {Key: "created_at", Value: -1}},
			Options: options.Index(),
		},
	})
	if err != nil {
		return err
	}

	// subscribers: unique compound index
	_, err = db.Collection(subscribersCollection).Indexes().CreateMany(ctx, []mongo.IndexModel{
		{
			Keys:    bson.D{{Key: "user_id", Value: 1}, {Key: "device_token", Value: 1}},
			Options: options.Index().SetUnique(true),
		},
		{
			Keys:    bson.D{{Key: "user_id", Value: 1}},
			Options: options.Index(),
		},
	})
	if err != nil {
		return err
	}

	// preferences: unique index on user_id
	_, err = db.Collection(preferencesCollection).Indexes().CreateMany(ctx, []mongo.IndexModel{
		{
			Keys:    bson.D{{Key: "user_id", Value: 1}},
			Options: options.Index().SetUnique(true),
		},
	})
	return err
}
