package mongo

import (
	"context"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/techinsight/be-techinsights-notification-service/internal/domain"
	"github.com/techinsight/be-techinsights-notification-service/internal/store"
)

const preferencesCollection = "preferences"

type preferenceStore struct {
	coll *mongo.Collection
}

// NewPreferenceStore returns a MongoDB-backed PreferenceStore.
func NewPreferenceStore(db *mongo.Database) store.PreferenceStore {
	return &preferenceStore{coll: db.Collection(preferencesCollection)}
}

func (s *preferenceStore) Get(ctx context.Context, userID string) (*domain.Preference, error) {
	var p domain.Preference
	err := s.coll.FindOne(ctx, bson.M{"user_id": userID}).Decode(&p)
	if err == mongo.ErrNoDocuments {
		return &domain.Preference{
			UserID:       userID,
			InAppEnabled: true,
			PushEnabled:  true,
		}, nil
	}
	if err != nil {
		return nil, err
	}
	return &p, nil
}

func (s *preferenceStore) Set(ctx context.Context, p *domain.Preference) error {
	filter := bson.M{"user_id": p.UserID}
	update := bson.M{"$set": p}
	opts := options.Update().SetUpsert(true)
	_, err := s.coll.UpdateOne(ctx, filter, update, opts)
	return err
}
