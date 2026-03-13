// internal/app/store/logdata/store.go
// Package logdata provides read-only access to the stratalog logdata collection.
package logdata

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

const collectionName = "logdata"

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
// Uses exact string matching for eventKeys.
func (s *Store) ScanTriggers(ctx context.Context, game string, triggerKeys []string, afterID primitive.ObjectID, limit int) ([]LogEntry, error) {
	filter := bson.M{
		"game":     game,
		"eventKey": bson.M{"$in": triggerKeys},
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
// Uses exact string matching for eventKey.
func (s *Store) CountByEventKey(ctx context.Context, game, playerID, eventKey string) (int64, error) {
	filter := bson.M{
		"game":     game,
		"playerId": playerID,
		"eventKey": eventKey,
	}
	return s.coll.CountDocuments(ctx, filter)
}

// CountByEventKeyAfter counts logs matching criteria after a given timestamp.
// Uses exact string matching for eventKey.
func (s *Store) CountByEventKeyAfter(ctx context.Context, game, playerID, eventKey string, after time.Time) (int64, error) {
	filter := bson.M{
		"game":            game,
		"playerId":        playerID,
		"eventKey":        eventKey,
		"serverTimestamp": bson.M{"$gte": after},
	}
	return s.coll.CountDocuments(ctx, filter)
}

// ExistsByEventKey checks if any log exists matching the criteria.
// Uses exact string matching for eventKey.
func (s *Store) ExistsByEventKey(ctx context.Context, game, playerID, eventKey string) (bool, error) {
	count, err := s.coll.CountDocuments(ctx, bson.M{
		"game":     game,
		"playerId": playerID,
		"eventKey": eventKey,
	}, options.Count().SetLimit(1))
	return count > 0, err
}

// ExistsByEventKeys checks if any log exists matching any of the given eventKeys.
// Uses exact string matching for eventKeys.
func (s *Store) ExistsByEventKeys(ctx context.Context, game, playerID string, eventKeys []string) (bool, error) {
	filter := bson.M{
		"game":     game,
		"playerId": playerID,
		"eventKey": bson.M{"$in": eventKeys},
	}
	count, err := s.coll.CountDocuments(ctx, filter, options.Count().SetLimit(1))
	return count > 0, err
}

// FindByEventKey finds all logs matching the criteria.
// Uses exact string matching for eventKey.
func (s *Store) FindByEventKey(ctx context.Context, game, playerID, eventKey string) ([]LogEntry, error) {
	filter := bson.M{
		"game":     game,
		"playerId": playerID,
		"eventKey": eventKey,
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
// Uses exact string matching for eventKey.
func (s *Store) GetMostRecent(ctx context.Context, game, playerID, eventKey string) (*LogEntry, error) {
	filter := bson.M{
		"game":     game,
		"playerId": playerID,
		"eventKey": eventKey,
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
// Uses exact string matching for eventKey.
func (s *Store) CountByEventKeyInWindow(ctx context.Context, game, playerID, eventKey string, windowStart time.Time) (int64, error) {
	filter := bson.M{
		"game":            game,
		"playerId":        playerID,
		"eventKey":        eventKey,
		"serverTimestamp": bson.M{"$gte": windowStart},
	}
	return s.coll.CountDocuments(ctx, filter)
}

// FindByEventKeyInWindow finds logs in a time window.
// Uses exact string matching for eventKey.
func (s *Store) FindByEventKeyInWindow(ctx context.Context, game, playerID, eventKey string, windowStart time.Time) ([]LogEntry, error) {
	filter := bson.M{
		"game":            game,
		"playerId":        playerID,
		"eventKey":        eventKey,
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

// ============================================================================
// _id-based windowing methods for replay-safe grading
// ============================================================================

// GetLatestByEventKey gets the most recent log by _id (not timestamp).
func (s *Store) GetLatestByEventKey(ctx context.Context, game, playerID, eventKey string) (*LogEntry, error) {
	filter := bson.M{
		"game":     game,
		"playerId": playerID,
		"eventKey": eventKey,
	}
	opts := options.FindOne().SetSort(bson.D{{Key: "_id", Value: -1}})

	var entry LogEntry
	err := s.coll.FindOne(ctx, filter, opts).Decode(&entry)
	if err == mongo.ErrNoDocuments {
		return nil, nil
	}
	return &entry, err
}

// GetPreviousByEventKey gets the most recent log before a given _id.
func (s *Store) GetPreviousByEventKey(ctx context.Context, game, playerID, eventKey string, beforeID primitive.ObjectID) (*LogEntry, error) {
	filter := bson.M{
		"game":     game,
		"playerId": playerID,
		"eventKey": eventKey,
		"_id":      bson.M{"$lt": beforeID},
	}
	opts := options.FindOne().SetSort(bson.D{{Key: "_id", Value: -1}})

	var entry LogEntry
	err := s.coll.FindOne(ctx, filter, opts).Decode(&entry)
	if err == mongo.ErrNoDocuments {
		return nil, nil
	}
	return &entry, err
}

// ExistsByEventKeyInIDWindow checks if event exists in _id range (startID, endID].
// The range is exclusive on startID and inclusive on endID.
func (s *Store) ExistsByEventKeyInIDWindow(ctx context.Context, game, playerID, eventKey string, startID, endID primitive.ObjectID) (bool, error) {
	filter := bson.M{
		"game":     game,
		"playerId": playerID,
		"eventKey": eventKey,
		"_id":      bson.M{"$gt": startID, "$lte": endID},
	}
	count, err := s.coll.CountDocuments(ctx, filter, options.Count().SetLimit(1))
	return count > 0, err
}

// ExistsByEventKeysInIDWindow checks if any event exists in _id range (startID, endID].
func (s *Store) ExistsByEventKeysInIDWindow(ctx context.Context, game, playerID string, eventKeys []string, startID, endID primitive.ObjectID) (bool, error) {
	filter := bson.M{
		"game":     game,
		"playerId": playerID,
		"eventKey": bson.M{"$in": eventKeys},
		"_id":      bson.M{"$gt": startID, "$lte": endID},
	}
	count, err := s.coll.CountDocuments(ctx, filter, options.Count().SetLimit(1))
	return count > 0, err
}

// CountByEventKeyInIDWindow counts events in _id range (startID, endID].
func (s *Store) CountByEventKeyInIDWindow(ctx context.Context, game, playerID, eventKey string, startID, endID primitive.ObjectID) (int64, error) {
	filter := bson.M{
		"game":     game,
		"playerId": playerID,
		"eventKey": eventKey,
		"_id":      bson.M{"$gt": startID, "$lte": endID},
	}
	return s.coll.CountDocuments(ctx, filter)
}

// CountByEventKeysInIDWindow counts events matching any key in _id range (startID, endID].
func (s *Store) CountByEventKeysInIDWindow(ctx context.Context, game, playerID string, eventKeys []string, startID, endID primitive.ObjectID) (int64, error) {
	filter := bson.M{
		"game":     game,
		"playerId": playerID,
		"eventKey": bson.M{"$in": eventKeys},
		"_id":      bson.M{"$gt": startID, "$lte": endID},
	}
	return s.coll.CountDocuments(ctx, filter)
}

// ExistsByEventTypeAndDataInIDWindow checks for eventType + data.field match in window.
func (s *Store) ExistsByEventTypeAndDataInIDWindow(ctx context.Context, game, playerID, eventType, dataField, dataValue string, startID, endID primitive.ObjectID) (bool, error) {
	filter := bson.M{
		"game":              game,
		"playerId":          playerID,
		"eventType":         eventType,
		"data." + dataField: dataValue,
		"_id":               bson.M{"$gt": startID, "$lte": endID},
	}
	count, err := s.coll.CountDocuments(ctx, filter, options.Count().SetLimit(1))
	return count > 0, err
}

// CountByEventTypeAndDataInIDWindow counts events matching eventType + data fields in window.
func (s *Store) CountByEventTypeAndDataInIDWindow(ctx context.Context, game, playerID, eventType string, dataFilter map[string]string, startID, endID primitive.ObjectID) (int64, error) {
	filter := bson.M{
		"game":      game,
		"playerId":  playerID,
		"eventType": eventType,
		"_id":       bson.M{"$gt": startID, "$lte": endID},
	}
	for field, value := range dataFilter {
		filter["data."+field] = value
	}
	return s.coll.CountDocuments(ctx, filter)
}

// FindByEventTypeAndDataInIDWindow finds events matching eventType + data fields in window.
// Returns sorted by _id descending (latest first).
func (s *Store) FindByEventTypeAndDataInIDWindow(ctx context.Context, game, playerID, eventType string, dataFilter map[string]string, startID, endID primitive.ObjectID) ([]LogEntry, error) {
	filter := bson.M{
		"game":      game,
		"playerId":  playerID,
		"eventType": eventType,
		"_id":       bson.M{"$gt": startID, "$lte": endID},
	}
	for field, value := range dataFilter {
		filter["data."+field] = value
	}
	opts := options.Find().SetSort(bson.D{{Key: "_id", Value: -1}})

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

// FindLatestByEventTypeAndDataInIDWindow finds the most recent event matching eventType + data in window.
func (s *Store) FindLatestByEventTypeAndDataInIDWindow(ctx context.Context, game, playerID, eventType string, dataFilter map[string]string, startID, endID primitive.ObjectID) (*LogEntry, error) {
	filter := bson.M{
		"game":      game,
		"playerId":  playerID,
		"eventType": eventType,
		"_id":       bson.M{"$gt": startID, "$lte": endID},
	}
	for field, value := range dataFilter {
		filter["data."+field] = value
	}
	opts := options.FindOne().SetSort(bson.D{{Key: "_id", Value: -1}})

	var entry LogEntry
	err := s.coll.FindOne(ctx, filter, opts).Decode(&entry)
	if err == mongo.ErrNoDocuments {
		return nil, nil
	}
	return &entry, err
}

// FindEventPairByEventTypeAndDataInIDWindow finds the first and last events matching
// eventType + data fields in a window. Used for timing calculations.
func (s *Store) FindEventPairByEventTypeAndDataInIDWindow(ctx context.Context, game, playerID, eventType string, firstDataFilter, lastDataFilter map[string]string, startID, endID primitive.ObjectID) (first *LogEntry, last *LogEntry, err error) {
	// Find first matching event
	firstFilter := bson.M{
		"game":      game,
		"playerId":  playerID,
		"eventType": eventType,
		"_id":       bson.M{"$gt": startID, "$lte": endID},
	}
	for field, value := range firstDataFilter {
		firstFilter["data."+field] = value
	}
	optsFirst := options.FindOne().SetSort(bson.D{{Key: "_id", Value: 1}})

	var firstEntry LogEntry
	err = s.coll.FindOne(ctx, firstFilter, optsFirst).Decode(&firstEntry)
	if err == mongo.ErrNoDocuments {
		return nil, nil, nil
	}
	if err != nil {
		return nil, nil, err
	}

	// Find last matching event
	lastFilter := bson.M{
		"game":      game,
		"playerId":  playerID,
		"eventType": eventType,
		"_id":       bson.M{"$gt": startID, "$lte": endID},
	}
	for field, value := range lastDataFilter {
		lastFilter["data."+field] = value
	}
	optsLast := options.FindOne().SetSort(bson.D{{Key: "_id", Value: -1}})

	var lastEntry LogEntry
	err = s.coll.FindOne(ctx, lastFilter, optsLast).Decode(&lastEntry)
	if err == mongo.ErrNoDocuments {
		return &firstEntry, nil, nil
	}
	if err != nil {
		return nil, nil, err
	}

	return &firstEntry, &lastEntry, nil
}
