package mongo

import (
	"context"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"be-modami-no-service/internal/domain"
	"be-modami-no-service/internal/store"
)

const subscribersCollection = "subscribers"

type subscriberStore struct {
	coll *mongo.Collection
}

// NewSubscriberStore returns a MongoDB-backed SubscriberStore.
func NewSubscriberStore(db *mongo.Database) store.SubscriberStore {
	return &subscriberStore{coll: db.Collection(subscribersCollection)}
}

func (s *subscriberStore) Upsert(ctx context.Context, sub *domain.Subscriber) error {
	filter := bson.M{"user_id": sub.UserID, "device_token": sub.DeviceToken}
	update := bson.M{"$set": sub}
	opts := options.Update().SetUpsert(true)
	_, err := s.coll.UpdateOne(ctx, filter, update, opts)
	return err
}

func (s *subscriberStore) ByUserID(ctx context.Context, userID string) ([]*domain.Subscriber, error) {
	cursor, err := s.coll.Find(ctx, bson.M{"user_id": userID})
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var results []*domain.Subscriber
	if err := cursor.All(ctx, &results); err != nil {
		return nil, err
	}
	if results == nil {
		results = []*domain.Subscriber{}
	}
	return results, nil
}

func (s *subscriberStore) DeleteByToken(ctx context.Context, userID, token string) error {
	_, err := s.coll.DeleteOne(ctx, bson.M{"user_id": userID, "device_token": token})
	return err
}
