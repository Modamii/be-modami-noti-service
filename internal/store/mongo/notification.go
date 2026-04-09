package mongo

import (
	"context"
	"math"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"be-modami-no-service/internal/domain"
	"be-modami-no-service/internal/store"
)

const notificationsCollection = "notifications"

type notificationStore struct {
	coll *mongo.Collection
}

// NewNotificationStore returns a MongoDB-backed NotificationStore.
func NewNotificationStore(db *mongo.Database) store.NotificationStore {
	return &notificationStore{coll: db.Collection(notificationsCollection)}
}

func (s *notificationStore) Create(ctx context.Context, n *domain.Notification) error {
	if n.CreatedAt.IsZero() {
		n.CreatedAt = time.Now()
	}
	_, err := s.coll.InsertOne(ctx, n)
	return err
}

func (s *notificationStore) GetByID(ctx context.Context, id string) (*domain.Notification, error) {
	var n domain.Notification
	err := s.coll.FindOne(ctx, bson.M{"_id": id}).Decode(&n)
	if err == mongo.ErrNoDocuments {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &n, nil
}

func (s *notificationStore) ListByUserID(ctx context.Context, userID string, limit int) ([]*domain.Notification, error) {
	if limit <= 0 {
		limit = 20
	}
	opts := options.Find().
		SetSort(bson.D{{Key: "created_at", Value: -1}}).
		SetLimit(int64(limit))

	cursor, err := s.coll.Find(ctx, bson.M{"user_id": userID}, opts)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var results []*domain.Notification
	if err := cursor.All(ctx, &results); err != nil {
		return nil, err
	}
	if results == nil {
		results = []*domain.Notification{}
	}
	return results, nil
}

func (s *notificationStore) ListByUserIDPaginated(ctx context.Context, userID string, params store.PaginationParams) (*store.PaginatedResult, error) {
	page := params.Page
	if page <= 0 {
		page = 1
	}
	perPage := params.PerPage
	if perPage <= 0 {
		perPage = 20
	}
	if perPage > 100 {
		perPage = 100
	}

	filter := bson.M{"user_id": userID}
	if params.UnreadOnly {
		filter["read"] = false
	}

	total, err := s.coll.CountDocuments(ctx, filter)
	if err != nil {
		return nil, err
	}

	skip := int64((page - 1) * perPage)
	opts := options.Find().
		SetSort(bson.D{{Key: "created_at", Value: -1}}).
		SetSkip(skip).
		SetLimit(int64(perPage))

	cursor, err := s.coll.Find(ctx, filter, opts)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var results []*domain.Notification
	if err := cursor.All(ctx, &results); err != nil {
		return nil, err
	}
	if results == nil {
		results = []*domain.Notification{}
	}

	totalPages := int(math.Ceil(float64(total) / float64(perPage)))

	return &store.PaginatedResult{
		Items:      results,
		Total:      total,
		Page:       page,
		PerPage:    perPage,
		TotalPages: totalPages,
		HasMore:    page < totalPages,
	}, nil
}

func (s *notificationStore) MarkRead(ctx context.Context, id string) error {
	_, err := s.coll.UpdateOne(ctx, bson.M{"_id": id}, bson.M{"$set": bson.M{"read": true}})
	return err
}

func (s *notificationStore) MarkAllRead(ctx context.Context, userID string) (int64, error) {
	result, err := s.coll.UpdateMany(ctx, bson.M{"user_id": userID, "read": false}, bson.M{"$set": bson.M{"read": true}})
	if err != nil {
		return 0, err
	}
	return result.ModifiedCount, nil
}

func (s *notificationStore) Delete(ctx context.Context, id string) error {
	_, err := s.coll.DeleteOne(ctx, bson.M{"_id": id})
	return err
}

func (s *notificationStore) CountUnread(ctx context.Context, userID string) (int64, error) {
	return s.coll.CountDocuments(ctx, bson.M{"user_id": userID, "read": false})
}
