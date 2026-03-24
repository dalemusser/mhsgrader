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
	Attempt            int            `bson:"attempt"`                      // 1-based attempt number
	Status             string         `bson:"status"`                       // "active", "passed", or "flagged"
	ComputedAt         time.Time      `bson:"computedAt"`                   // When grade was computed
	RuleID             string         `bson:"ruleId"`                       // e.g., "u1p1_v2"
	ReasonCode         string         `bson:"reasonCode,omitempty"`         // e.g., "TOO_MANY_TARGETS"
	Metrics            map[string]any `bson:"metrics,omitempty"`            // e.g., {countTargets: 9, threshold: 6}
	StartTime          *time.Time     `bson:"startTime,omitempty"`          // Activity start
	EndTime            *time.Time     `bson:"endTime,omitempty"`            // Activity end
	DurationSecs       *float64       `bson:"durationSecs,omitempty"`       // Wall-clock time to complete (seconds)
	ActiveDurationSecs *float64       `bson:"activeDurationSecs,omitempty"` // Active time excluding gaps (seconds)
}

// PlayerGrades represents all grades for a single player.
type PlayerGrades struct {
	Game        string             `bson:"game"`                  // Game identifier
	PlayerID    string             `bson:"playerId"`              // Player identifier
	Grades      map[string][]Grade `bson:"grades"`                // Map of point ID to array of attempt grades
	CurrentUnit string             `bson:"currentUnit,omitempty"` // Unit the student is currently in
	LastUpdated time.Time          `bson:"lastUpdated"`           // When document was last modified
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

// AppendGrade appends or replaces the latest grade for a progress point.
// If the last element for this point has status "active", it is replaced with the final grade
// (preserving the attempt number). Otherwise, a new grade is appended.
func (s *Store) AppendGrade(ctx context.Context, game, playerID, pointID string, grade Grade) error {
	now := time.Now().UTC()
	grade.ComputedAt = now

	// Read current doc to determine array state
	pg, err := s.GetForPlayer(ctx, game, playerID)
	if err != nil {
		return err
	}

	filter := bson.M{"game": game, "playerId": playerID}

	if pg != nil {
		grades := pg.Grades[pointID]
		if len(grades) > 0 {
			last := grades[len(grades)-1]
			if last.Status == "active" {
				// Replace the active entry, preserving its attempt number
				grade.Attempt = last.Attempt
				idx := len(grades) - 1
				update := bson.M{
					"$set": bson.M{
						"grades." + pointID + "." + itoa(idx): grade,
						"lastUpdated": now,
					},
				}
				_, err := s.coll.UpdateOne(ctx, filter, update)
				return err
			}
			// Append new grade with next attempt number
			grade.Attempt = len(grades) + 1
		} else {
			// No array for this point yet
			grade.Attempt = 1
		}

		update := bson.M{
			"$push": bson.M{"grades." + pointID: grade},
			"$set":  bson.M{"lastUpdated": now},
		}
		_, err := s.coll.UpdateOne(ctx, filter, update)
		return err
	}

	// No doc exists — create with first grade
	grade.Attempt = 1
	doc := PlayerGrades{
		Game:        game,
		PlayerID:    playerID,
		Grades:      map[string][]Grade{pointID: {grade}},
		LastUpdated: now,
	}
	_, err = s.coll.InsertOne(ctx, doc)
	return err
}

// AppendActiveIfNeeded appends an "active" grade if the point needs one.
// No-op if the last element is already "active".
// Appends a new active grade if last element is "passed" or "flagged" (new attempt).
// Creates doc with active grade if no doc exists.
func (s *Store) AppendActiveIfNeeded(ctx context.Context, game, playerID, pointID, ruleID string, startTime *time.Time) error {
	now := time.Now().UTC()

	pg, err := s.GetForPlayer(ctx, game, playerID)
	if err != nil {
		return err
	}

	grade := Grade{
		Status:     "active",
		ComputedAt: now,
		RuleID:     ruleID,
		StartTime:  startTime,
	}

	filter := bson.M{"game": game, "playerId": playerID}

	if pg != nil {
		grades := pg.Grades[pointID]
		if len(grades) > 0 {
			last := grades[len(grades)-1]
			if last.Status == "active" {
				// Already active — no-op
				return nil
			}
			// Last is passed/flagged — start new attempt
			grade.Attempt = len(grades) + 1
		} else {
			grade.Attempt = 1
		}

		update := bson.M{
			"$push": bson.M{"grades." + pointID: grade},
			"$set":  bson.M{"lastUpdated": now},
		}
		_, err := s.coll.UpdateOne(ctx, filter, update)
		return err
	}

	// No doc — create
	grade.Attempt = 1
	doc := PlayerGrades{
		Game:        game,
		PlayerID:    playerID,
		Grades:      map[string][]Grade{pointID: {grade}},
		LastUpdated: now,
	}
	_, err = s.coll.InsertOne(ctx, doc)
	return err
}

// SetCurrentUnit updates the unit the student is currently in.
func (s *Store) SetCurrentUnit(ctx context.Context, game, playerID, unitID string) error {
	now := time.Now().UTC()

	filter := bson.M{"game": game, "playerId": playerID}
	update := bson.M{
		"$set": bson.M{
			"currentUnit": unitID,
			"lastUpdated": now,
		},
		"$setOnInsert": bson.M{
			"game":     game,
			"playerId": playerID,
		},
	}

	_, err := s.coll.UpdateOne(ctx, filter, update, options.Update().SetUpsert(true))
	return err
}

// GetLatestGrade retrieves the latest grade for a specific point.
func (s *Store) GetLatestGrade(ctx context.Context, game, playerID, pointID string) (*Grade, error) {
	pg, err := s.GetForPlayer(ctx, game, playerID)
	if err != nil || pg == nil {
		return nil, err
	}
	grades := pg.Grades[pointID]
	if len(grades) == 0 {
		return nil, nil
	}
	latest := grades[len(grades)-1]
	return &latest, nil
}

// GetGradeHistory retrieves all grades for a specific point.
func (s *Store) GetGradeHistory(ctx context.Context, game, playerID, pointID string) ([]Grade, error) {
	pg, err := s.GetForPlayer(ctx, game, playerID)
	if err != nil || pg == nil {
		return nil, err
	}
	return pg.Grades[pointID], nil
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

// DeleteByGame removes all grade documents for a specific game. Returns count of deleted documents.
func (s *Store) DeleteByGame(ctx context.Context, game string) (int64, error) {
	result, err := s.coll.DeleteMany(ctx, bson.M{"game": game})
	if err != nil {
		return 0, err
	}
	return result.DeletedCount, nil
}

// itoa converts an int to a string (for building BSON paths).
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	s := ""
	for n > 0 {
		s = string(rune('0'+n%10)) + s
		n /= 10
	}
	return s
}
