// internal/app/store/graderstate/store.go
// Package graderstate manages cursor persistence for the grading engine.
package graderstate

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// State represents the grader's processing state.
type State struct {
	ID         string             `bson:"_id"`              // e.g., "mhs-grader"
	LastSeenID primitive.ObjectID `bson:"lastSeenId"`       // Last processed log _id
	UpdatedAt  time.Time          `bson:"updatedAt"`        // When state was last updated
}

// Store handles grader state persistence.
type Store struct {
	coll *mongo.Collection
}

// New creates a new grader state store.
func New(db *mongo.Database) *Store {
	return &Store{coll: db.Collection("grader_state")}
}

// Get retrieves the current grader state.
// Returns a zero State if not found.
func (s *Store) Get(ctx context.Context, graderID string) (State, error) {
	var state State
	err := s.coll.FindOne(ctx, bson.M{"_id": graderID}).Decode(&state)
	if err == mongo.ErrNoDocuments {
		return State{ID: graderID}, nil
	}
	return state, err
}

// UpdateLastSeenID updates the last seen log ID.
func (s *Store) UpdateLastSeenID(ctx context.Context, graderID string, lastSeenID primitive.ObjectID) error {
	_, err := s.coll.UpdateOne(
		ctx,
		bson.M{"_id": graderID},
		bson.M{
			"$set": bson.M{
				"lastSeenId": lastSeenID,
				"updatedAt":  time.Now().UTC(),
			},
		},
		options.Update().SetUpsert(true),
	)
	return err
}

// Reset clears the grader state (for backfill mode).
func (s *Store) Reset(ctx context.Context, graderID string) error {
	_, err := s.coll.DeleteOne(ctx, bson.M{"_id": graderID})
	return err
}
