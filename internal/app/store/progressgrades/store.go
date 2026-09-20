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

// Reason is one triggered reason code together with the variables its
// instructor-message template interpolates. Variable names are the
// specification's placeholders verbatim (e.g. "attempt_number").
type Reason struct {
	Code      string         `bson:"code"`
	Variables map[string]any `bson:"variables,omitempty"`
}

// Grade represents a single progress point grade.
type Grade struct {
	Attempt            int            `bson:"attempt"`                      // 1-based attempt number
	Status             string         `bson:"status"`                       // "active", "passed", or "flagged"
	ComputedAt         time.Time      `bson:"computedAt"`                   // When grade was computed
	RuleID             string         `bson:"ruleId"`                       // e.g., "u1p1_v3"
	ReasonCode         string         `bson:"reasonCode,omitempty"`         // First triggered code (kept for readers that predate Reasons)
	Reasons            []Reason       `bson:"reasons,omitempty"`            // Every triggered reason code with its message variables
	Metrics            map[string]any `bson:"metrics,omitempty"`            // Raw counts/scores behind the grade (analytics)
	StartTime          *time.Time     `bson:"startTime,omitempty"`          // Activity start
	EndTime            *time.Time     `bson:"endTime,omitempty"`            // Activity end
	DurationSecs       *float64       `bson:"durationSecs,omitempty"`       // Wall-clock time to complete (seconds)
	ActiveDurationSecs *float64       `bson:"activeDurationSecs,omitempty"` // Active time excluding gaps (seconds)
}

// UserGrades represents all grades for a single user.
type UserGrades struct {
	Game        string             `bson:"game"`                  // Game identifier
	UserID      string             `bson:"user_id"`               // User identifier
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

// GetForUser retrieves grades for a user.
func (s *Store) GetForUser(ctx context.Context, game, userID string) (*UserGrades, error) {
	var pg UserGrades
	err := s.coll.FindOne(ctx, bson.M{"game": game, "user_id": userID}).Decode(&pg)
	if err == mongo.ErrNoDocuments {
		return nil, nil
	}
	return &pg, err
}

// AppendGrade appends or replaces the latest grade for a progress point.
// If the last element for this point has status "active", it is replaced with the final grade
// (preserving the attempt number). Otherwise, a new grade is appended.
func (s *Store) AppendGrade(ctx context.Context, game, userID, pointID string, grade Grade) error {
	now := time.Now().UTC()
	grade.ComputedAt = now

	// Read current doc to determine array state
	pg, err := s.GetForUser(ctx, game, userID)
	if err != nil {
		return err
	}

	filter := bson.M{"game": game, "user_id": userID}

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
						"lastUpdated":                         now,
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
	doc := UserGrades{
		Game:        game,
		UserID:      userID,
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
func (s *Store) AppendActiveIfNeeded(ctx context.Context, game, userID, pointID, ruleID string, startTime *time.Time) error {
	now := time.Now().UTC()

	pg, err := s.GetForUser(ctx, game, userID)
	if err != nil {
		return err
	}

	grade := Grade{
		Status:     "active",
		ComputedAt: now,
		RuleID:     ruleID,
		StartTime:  startTime,
	}

	filter := bson.M{"game": game, "user_id": userID}

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
	doc := UserGrades{
		Game:        game,
		UserID:      userID,
		Grades:      map[string][]Grade{pointID: {grade}},
		LastUpdated: now,
	}
	_, err = s.coll.InsertOne(ctx, doc)
	return err
}

// SetCurrentUnit updates the unit the student is currently in.
func (s *Store) SetCurrentUnit(ctx context.Context, game, userID, unitID string) error {
	now := time.Now().UTC()

	filter := bson.M{"game": game, "user_id": userID}
	update := bson.M{
		"$set": bson.M{
			"currentUnit": unitID,
			"lastUpdated": now,
		},
		"$setOnInsert": bson.M{
			"game":    game,
			"user_id": userID,
		},
	}

	_, err := s.coll.UpdateOne(ctx, filter, update, options.Update().SetUpsert(true))
	return err
}

// GetLatestGrade retrieves the latest grade for a specific point.
func (s *Store) GetLatestGrade(ctx context.Context, game, userID, pointID string) (*Grade, error) {
	pg, err := s.GetForUser(ctx, game, userID)
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
func (s *Store) GetGradeHistory(ctx context.Context, game, userID, pointID string) ([]Grade, error) {
	pg, err := s.GetForUser(ctx, game, userID)
	if err != nil || pg == nil {
		return nil, err
	}
	return pg.Grades[pointID], nil
}

// ListUsers returns all user IDs that have grades for a game.
func (s *Store) ListUsers(ctx context.Context, game string) ([]string, error) {
	cur, err := s.coll.Find(ctx, bson.M{"game": game}, options.Find().SetProjection(bson.M{"user_id": 1}))
	if err != nil {
		return nil, err
	}
	defer cur.Close(ctx)

	var players []string
	for cur.Next(ctx) {
		var doc struct {
			UserID string `bson:"user_id"`
		}
		if err := cur.Decode(&doc); err != nil {
			continue
		}
		players = append(players, doc.UserID)
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
