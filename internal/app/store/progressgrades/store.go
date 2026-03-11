// internal/app/store/progressgrades/store.go
// Package progressgrades manages progress point grades storage.
package progressgrades

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// Grade represents a single progress point grade.
type Grade struct {
	Color      string         `bson:"color"`               // "green" or "yellow"
	ComputedAt time.Time      `bson:"computedAt"`          // When grade was computed
	RuleID     string         `bson:"ruleId"`              // e.g., "u1p1_v1"
	ReasonCode string         `bson:"reasonCode,omitempty"` // e.g., "TOO_MANY_TARGETS"
	Metrics    map[string]any `bson:"metrics,omitempty"`   // e.g., {countTargets: 9, threshold: 6}
}

// PlayerGrades represents all grades for a single player.
type PlayerGrades struct {
	Game        string           `bson:"game"`        // Game identifier
	PlayerID    string           `bson:"playerId"`    // Player identifier
	Grades      map[string]Grade `bson:"grades"`      // Map of point ID to grade
	LastUpdated time.Time        `bson:"lastUpdated"` // When document was last modified
}

// Store handles progress grades persistence.
type Store struct {
	coll *mongo.Collection
}

// New creates a new progress grades store.
func New(db *mongo.Database) *Store {
	return &Store{coll: db.Collection("progress_point_grades")}
}

// GetForPlayer retrieves grades for a player.
func (s *Store) GetForPlayer(ctx context.Context, game, playerID string) (*PlayerGrades, error) {
	var pg PlayerGrades
	err := s.coll.FindOne(ctx, bson.M{"game": game, "playerId": playerID}).Decode(&pg)
	if err == mongo.ErrNoDocuments {
		return nil, nil
	}
	return &pg, err
}

// UpsertGrade updates or inserts a grade for a specific progress point.
func (s *Store) UpsertGrade(ctx context.Context, game, playerID, pointID string, grade Grade) error {
	now := time.Now().UTC()
	grade.ComputedAt = now

	filter := bson.M{"game": game, "playerId": playerID}
	update := bson.M{
		"$set": bson.M{
			"grades." + pointID: grade,
			"lastUpdated":       now,
		},
		"$setOnInsert": bson.M{
			"game":     game,
			"playerId": playerID,
		},
	}

	_, err := s.coll.UpdateOne(ctx, filter, update, options.Update().SetUpsert(true))
	return err
}

// GetGrade retrieves a specific grade for a player.
func (s *Store) GetGrade(ctx context.Context, game, playerID, pointID string) (*Grade, error) {
	pg, err := s.GetForPlayer(ctx, game, playerID)
	if err != nil || pg == nil {
		return nil, err
	}
	if grade, ok := pg.Grades[pointID]; ok {
		return &grade, nil
	}
	return nil, nil
}

// ListPlayers returns all player IDs that have grades for a game.
func (s *Store) ListPlayers(ctx context.Context, game string) ([]string, error) {
	cur, err := s.coll.Find(ctx, bson.M{"game": game}, options.Find().SetProjection(bson.M{"playerId": 1}))
	if err != nil {
		return nil, err
	}
	defer cur.Close(ctx)

	var players []string
	for cur.Next(ctx) {
		var doc struct {
			PlayerID string `bson:"playerId"`
		}
		if err := cur.Decode(&doc); err != nil {
			continue
		}
		players = append(players, doc.PlayerID)
	}
	return players, cur.Err()
}

// DeleteAll removes all grade documents. Returns count of deleted documents.
func (s *Store) DeleteAll(ctx context.Context) (int64, error) {
	result, err := s.coll.DeleteMany(ctx, bson.M{})
	if err != nil {
		return 0, err
	}
	return result.DeletedCount, nil
}
