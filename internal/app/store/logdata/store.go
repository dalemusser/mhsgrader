// internal/app/store/logdata/store.go
// Package logdata provides read-only access to the stratalog logdata collection.
package logdata

import (
	"context"
	"regexp"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

const collectionName = "logdata"

// eventKeyFilter returns a case-insensitive filter for a single eventKey.
// This handles the case where eventKeys in the database may have different casing
// (e.g., "QuestFinishEvent:34" vs "questFinishEvent:34").
// Uses primitive.Regex for DocumentDB compatibility.
func eventKeyFilter(eventKey string) primitive.Regex {
	// Escape regex special characters in the eventKey
	escaped := regexp.QuoteMeta(eventKey)
	return primitive.Regex{Pattern: "^" + escaped + "$", Options: "i"}
}

// eventKeysFilter returns a case-insensitive filter for multiple eventKeys using $or.
func eventKeysFilter(eventKeys []string) bson.M {
	if len(eventKeys) == 1 {
		return bson.M{"eventKey": eventKeyFilter(eventKeys[0])}
	}
	orFilters := make([]bson.M, len(eventKeys))
	for i, key := range eventKeys {
		escaped := regexp.QuoteMeta(key)
		orFilters[i] = bson.M{"eventKey": primitive.Regex{Pattern: "^" + escaped + "$", Options: "i"}}
	}
	return bson.M{"$or": orFilters}
}

// LogEntry represents a log entry from stratalog.
type LogEntry struct {
	ID              primitive.ObjectID     `bson:"_id"`
	Game            string                 `bson:"game"`
	PlayerID        string                 `bson:"playerId,omitempty"`
	EventType       string                 `bson:"eventType,omitempty"`
	EventKey        string                 `bson:"eventKey,omitempty"` // For grading triggers
	Timestamp       *time.Time             `bson:"timestamp,omitempty"`
	ServerTimestamp time.Time              `bson:"serverTimestamp"`
	Data            map[string]interface{} `bson:"data,omitempty"`
}

// Store provides read-only access to logdata.
type Store struct {
	coll *mongo.Collection
}

// New creates a new logdata store.
func New(db *mongo.Database) *Store {
	return &Store{coll: db.Collection(collectionName)}
}

// ScanTriggers scans for log entries with specific eventKeys after a given _id.
// Returns entries sorted by _id ascending.
// Uses case-insensitive matching for eventKeys.
func (s *Store) ScanTriggers(ctx context.Context, game string, triggerKeys []string, afterID primitive.ObjectID, limit int) ([]LogEntry, error) {
	// Build base filter with game
	filter := bson.M{"game": game}

	// Add case-insensitive eventKey matching
	eventFilter := eventKeysFilter(triggerKeys)
	if orFilters, ok := eventFilter["$or"]; ok {
		filter["$or"] = orFilters
	} else if eventKey, ok := eventFilter["eventKey"]; ok {
		filter["eventKey"] = eventKey
	}

	// If afterID is not zero, only get entries after it
	if !afterID.IsZero() {
		filter["_id"] = bson.M{"$gt": afterID}
	}

	opts := options.Find().
		SetSort(bson.D{{Key: "_id", Value: 1}}).
		SetLimit(int64(limit))

	cur, err := s.coll.Find(ctx, filter, opts)
	if err != nil {
		return nil, err
	}
	defer cur.Close(ctx)

	var entries []LogEntry
	if err := cur.All(ctx, &entries); err != nil {
		return nil, err
	}
	return entries, nil
}

// CountByEventKey counts logs matching a game, player, and eventKey.
// Uses case-insensitive matching for eventKey.
func (s *Store) CountByEventKey(ctx context.Context, game, playerID, eventKey string) (int64, error) {
	filter := bson.M{
		"game":     game,
		"playerId": playerID,
		"eventKey": eventKeyFilter(eventKey),
	}
	return s.coll.CountDocuments(ctx, filter)
}

// CountByEventKeyAfter counts logs matching criteria after a given timestamp.
// Uses case-insensitive matching for eventKey.
func (s *Store) CountByEventKeyAfter(ctx context.Context, game, playerID, eventKey string, after time.Time) (int64, error) {
	filter := bson.M{
		"game":            game,
		"playerId":        playerID,
		"eventKey":        eventKeyFilter(eventKey),
		"serverTimestamp": bson.M{"$gte": after},
	}
	return s.coll.CountDocuments(ctx, filter)
}

// ExistsByEventKey checks if any log exists matching the criteria.
// Uses case-insensitive matching for eventKey.
func (s *Store) ExistsByEventKey(ctx context.Context, game, playerID, eventKey string) (bool, error) {
	count, err := s.coll.CountDocuments(ctx, bson.M{
		"game":     game,
		"playerId": playerID,
		"eventKey": eventKeyFilter(eventKey),
	}, options.Count().SetLimit(1))
	return count > 0, err
}

// ExistsByEventKeys checks if any log exists matching any of the given eventKeys.
// Uses case-insensitive matching for eventKeys.
func (s *Store) ExistsByEventKeys(ctx context.Context, game, playerID string, eventKeys []string) (bool, error) {
	filter := bson.M{
		"game":     game,
		"playerId": playerID,
	}
	// Add case-insensitive eventKey matching
	eventFilter := eventKeysFilter(eventKeys)
	if orFilters, ok := eventFilter["$or"]; ok {
		filter["$or"] = orFilters
	} else if eventKey, ok := eventFilter["eventKey"]; ok {
		filter["eventKey"] = eventKey
	}
	count, err := s.coll.CountDocuments(ctx, filter, options.Count().SetLimit(1))
	return count > 0, err
}

// FindByEventKey finds all logs matching the criteria.
// Uses case-insensitive matching for eventKey.
func (s *Store) FindByEventKey(ctx context.Context, game, playerID, eventKey string) ([]LogEntry, error) {
	filter := bson.M{
		"game":     game,
		"playerId": playerID,
		"eventKey": eventKeyFilter(eventKey),
	}
	opts := options.Find().SetSort(bson.D{{Key: "serverTimestamp", Value: 1}})

	cur, err := s.coll.Find(ctx, filter, opts)
	if err != nil {
		return nil, err
	}
	defer cur.Close(ctx)

	var entries []LogEntry
	if err := cur.All(ctx, &entries); err != nil {
		return nil, err
	}
	return entries, nil
}

// GetMostRecent gets the most recent log for a player with a specific eventKey.
// Uses case-insensitive matching for eventKey.
func (s *Store) GetMostRecent(ctx context.Context, game, playerID, eventKey string) (*LogEntry, error) {
	filter := bson.M{
		"game":     game,
		"playerId": playerID,
		"eventKey": eventKeyFilter(eventKey),
	}
	opts := options.FindOne().SetSort(bson.D{{Key: "serverTimestamp", Value: -1}})

	var entry LogEntry
	err := s.coll.FindOne(ctx, filter, opts).Decode(&entry)
	if err == mongo.ErrNoDocuments {
		return nil, nil
	}
	return &entry, err
}

// GetWindowStart finds the timestamp of an event that marks the start of a grading window.
func (s *Store) GetWindowStart(ctx context.Context, game, playerID, eventKey string) (*time.Time, error) {
	entry, err := s.GetMostRecent(ctx, game, playerID, eventKey)
	if err != nil || entry == nil {
		return nil, err
	}
	return &entry.ServerTimestamp, nil
}

// CountByEventKeyInWindow counts logs in a time window.
// Uses case-insensitive matching for eventKey.
func (s *Store) CountByEventKeyInWindow(ctx context.Context, game, playerID, eventKey string, windowStart time.Time) (int64, error) {
	filter := bson.M{
		"game":            game,
		"playerId":        playerID,
		"eventKey":        eventKeyFilter(eventKey),
		"serverTimestamp": bson.M{"$gte": windowStart},
	}
	return s.coll.CountDocuments(ctx, filter)
}

// FindByEventKeyInWindow finds logs in a time window.
// Uses case-insensitive matching for eventKey.
func (s *Store) FindByEventKeyInWindow(ctx context.Context, game, playerID, eventKey string, windowStart time.Time) ([]LogEntry, error) {
	filter := bson.M{
		"game":            game,
		"playerId":        playerID,
		"eventKey":        eventKeyFilter(eventKey),
		"serverTimestamp": bson.M{"$gte": windowStart},
	}
	opts := options.Find().SetSort(bson.D{{Key: "serverTimestamp", Value: 1}})

	cur, err := s.coll.Find(ctx, filter, opts)
	if err != nil {
		return nil, err
	}
	defer cur.Close(ctx)

	var entries []LogEntry
	if err := cur.All(ctx, &entries); err != nil {
		return nil, err
	}
	return entries, nil
}
