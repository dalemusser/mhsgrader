// internal/app/store/logdata/store.go
// Package logdata provides read-only access to the stratalog logdata collection.
package logdata

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/bsontype"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

const collectionName = "logdata"

// LogEntry represents a log entry from stratalog.
type LogEntry struct {
	ID              primitive.ObjectID `bson:"_id"`
	Game            string             `bson:"game"`
	UserID          string             `bson:"user_id,omitempty"`
	EventType       string             `bson:"eventType,omitempty"`
	EventKey        string             `bson:"eventKey,omitempty"`  // For grading triggers
	Timestamp       bson.RawValue      `bson:"timestamp,omitempty"` // Client timestamp exactly as stored (a string on current builds)
	ServerTimestamp time.Time          `bson:"serverTimestamp"`
	Data            map[string]any     `bson:"-"` // event payload; see UnmarshalBSON
}

// logEntryWire is LogEntry as stored, with the payload left undecoded.
type logEntryWire struct {
	ID              primitive.ObjectID `bson:"_id"`
	Game            string             `bson:"game"`
	UserID          string             `bson:"user_id,omitempty"`
	EventType       string             `bson:"eventType,omitempty"`
	EventKey        string             `bson:"eventKey,omitempty"`
	Timestamp       bson.RawValue      `bson:"timestamp,omitempty"`
	ServerTimestamp time.Time          `bson:"serverTimestamp"`
	Data            bson.RawValue      `bson:"data,omitempty"`
}

// UnmarshalBSON decodes an entry tolerantly. The game has logged `data` as a
// JSON string on some builds instead of an object; a string payload is parsed
// as JSON when it is one and otherwise kept under "_raw", so one odd record
// can never stop a batch from decoding.
func (e *LogEntry) UnmarshalBSON(b []byte) error {
	var w logEntryWire
	if err := bson.Unmarshal(b, &w); err != nil {
		return err
	}
	*e = LogEntry{
		ID: w.ID, Game: w.Game, UserID: w.UserID, EventType: w.EventType, EventKey: w.EventKey,
		Timestamp: w.Timestamp, ServerTimestamp: w.ServerTimestamp,
	}
	switch w.Data.Type {
	case bsontype.EmbeddedDocument:
		var m map[string]any
		if err := w.Data.Unmarshal(&m); err == nil {
			e.Data = m
		}
	case bsontype.String:
		s := w.Data.StringValue()
		var m map[string]any
		if err := json.Unmarshal([]byte(s), &m); err == nil {
			e.Data = m
		} else if s != "" {
			e.Data = map[string]any{"_raw": s}
		}
	}
	return nil
}

// HasTimestamp reports whether the entry carries a client timestamp.
func (e *LogEntry) HasTimestamp() bool {
	return e.Timestamp.Type != 0 && e.Timestamp.Type != bsontype.Null && e.Timestamp.Type != bsontype.Undefined
}

// Match selects log entries either by exact eventKey or by eventType plus
// equality/regex conditions on data fields. A Match with an EventKey ignores
// the other fields.
type Match struct {
	EventKey  string
	EventType string
	Data      map[string]any // e.g. {"Soil Key Puzzle Status": "Finished", "Unit": primitive.Regex{Pattern: "^Unit 4"}}
}

// KeyMatch returns a Match on an exact eventKey.
func KeyMatch(key string) Match { return Match{EventKey: key} }

// TypeMatch returns a Match on eventType plus data-field conditions.
func TypeMatch(eventType string, data map[string]any) Match {
	return Match{EventType: eventType, Data: data}
}

// String describes the match for logs and grade metrics.
func (m Match) String() string {
	if m.EventKey != "" {
		return m.EventKey
	}
	return fmt.Sprintf("%s%v", m.EventType, m.Data)
}

func (m Match) filter() bson.M {
	if m.EventKey != "" {
		return bson.M{"eventKey": m.EventKey}
	}
	f := bson.M{"eventType": m.EventType}
	for k, v := range m.Data {
		f["data."+k] = v
	}
	return f
}

// Matches tests an already-loaded entry against the match, mirroring the
// query semantics for string equality and regex conditions.
func (m Match) Matches(e *LogEntry) bool {
	if m.EventKey != "" {
		return e.EventKey == m.EventKey
	}
	if e.EventType != m.EventType {
		return false
	}
	for k, want := range m.Data {
		got, ok := e.Data[k]
		if !ok {
			return false
		}
		switch w := want.(type) {
		case string:
			if s, ok := got.(string); !ok || s != w {
				return false
			}
		case primitive.Regex:
			s, ok := got.(string)
			if !ok {
				return false
			}
			re, err := regexp.Compile(w.Pattern)
			if err != nil || !re.MatchString(s) {
				return false
			}
		default:
			if fmt.Sprint(got) != fmt.Sprint(want) {
				return false
			}
		}
	}
	return true
}

// anyOf adds to f a condition matching any of the keys or matches.
func anyOf(f bson.M, keys []string, matches []Match) {
	var or []bson.M
	if len(keys) > 0 {
		or = append(or, bson.M{"eventKey": bson.M{"$in": keys}})
	}
	for _, m := range matches {
		or = append(or, m.filter())
	}
	switch len(or) {
	case 0:
		f["eventKey"] = bson.M{"$in": []string{}} // matches nothing
	case 1:
		for k, v := range or[0] {
			f[k] = v
		}
	default:
		f["$or"] = or
	}
}

// Window bounds a query to the _id range (StartID, EndID]; when both
// timestamp bounds are set the client `timestamp` field is fenced as well
// (inclusive on both ends, compared as stored).
type Window struct {
	StartID primitive.ObjectID // exclusive
	EndID   primitive.ObjectID // inclusive
	TSStart *bson.RawValue
	TSEnd   *bson.RawValue
}

func (w Window) apply(f bson.M) {
	f["_id"] = bson.M{"$gt": w.StartID, "$lte": w.EndID}
	if w.TSStart != nil && w.TSEnd != nil {
		f["timestamp"] = bson.M{"$gte": *w.TSStart, "$lte": *w.TSEnd}
	}
}

// Store provides read-only access to logdata.
type Store struct {
	coll *mongo.Collection
}

// New creates a new logdata store.
func New(db *mongo.Database) *Store {
	return &Store{coll: db.Collection(collectionName)}
}

func base(game, userID string) bson.M {
	return bson.M{"game": game, "user_id": userID}
}

// ScanTriggers returns entries after afterID (by _id, ascending) that match
// any of the trigger keys or matches, up to limit.
func (s *Store) ScanTriggers(ctx context.Context, game string, triggerKeys []string, matches []Match, afterID primitive.ObjectID, limit int) ([]LogEntry, error) {
	filter := bson.M{"game": game}
	anyOf(filter, triggerKeys, matches)
	if !afterID.IsZero() {
		filter["_id"] = bson.M{"$gt": afterID}
	}
	opts := options.Find().SetSort(bson.D{{Key: "_id", Value: 1}}).SetLimit(int64(limit))
	cur, err := s.coll.Find(ctx, filter, opts)
	if err != nil {
		return nil, err
	}
	defer cur.Close(ctx)
	var entries []LogEntry
	for cur.Next(ctx) {
		var e LogEntry
		if err := cur.Decode(&e); err != nil {
			// Keep the record in the batch with what can be read raw, so the
			// cursor still moves past it; nothing dispatches on it if the key
			// fields are unreadable.
			e = LogEntry{}
			if id, ok := cur.Current.Lookup("_id").ObjectIDOK(); ok {
				e.ID = id
			}
			if v, ok := cur.Current.Lookup("user_id").StringValueOK(); ok {
				e.UserID = v
			}
			if v, ok := cur.Current.Lookup("eventKey").StringValueOK(); ok {
				e.EventKey = v
			}
			if v, ok := cur.Current.Lookup("eventType").StringValueOK(); ok {
				e.EventType = v
			}
			if t, ok := cur.Current.Lookup("serverTimestamp").TimeOK(); ok {
				e.ServerTimestamp = t
			}
			if e.ID.IsZero() {
				continue
			}
		}
		entries = append(entries, e)
	}
	return entries, cur.Err()
}

// LatestBefore returns the most recent entry (by _id) matching any of the
// keys or matches with _id < beforeID, or nil.
func (s *Store) LatestBefore(ctx context.Context, game, userID string, keys []string, matches []Match, beforeID primitive.ObjectID) (*LogEntry, error) {
	f := base(game, userID)
	anyOf(f, keys, matches)
	f["_id"] = bson.M{"$lt": beforeID}
	return s.findOne(ctx, f, bson.D{{Key: "_id", Value: -1}})
}

// GetLatestByEventKeysBefore is LatestBefore restricted to eventKeys.
func (s *Store) GetLatestByEventKeysBefore(ctx context.Context, game, userID string, eventKeys []string, beforeID primitive.ObjectID) (*LogEntry, error) {
	return s.LatestBefore(ctx, game, userID, eventKeys, nil, beforeID)
}

// ExistsInWindow reports whether any entry with one of the keys lies in w.
func (s *Store) ExistsInWindow(ctx context.Context, game, userID string, keys []string, w Window) (bool, error) {
	f := base(game, userID)
	f["eventKey"] = bson.M{"$in": keys}
	w.apply(f)
	n, err := s.coll.CountDocuments(ctx, f, options.Count().SetLimit(1))
	return n > 0, err
}

// CountInWindow counts entries with one of the keys in w.
func (s *Store) CountInWindow(ctx context.Context, game, userID string, keys []string, w Window) (int64, error) {
	f := base(game, userID)
	f["eventKey"] = bson.M{"$in": keys}
	w.apply(f)
	return s.coll.CountDocuments(ctx, f)
}

// FindInWindow returns entries with one of the keys in w, ascending by _id.
func (s *Store) FindInWindow(ctx context.Context, game, userID string, keys []string, w Window) ([]LogEntry, error) {
	f := base(game, userID)
	f["eventKey"] = bson.M{"$in": keys}
	w.apply(f)
	return s.find(ctx, f, options.Find().SetSort(bson.D{{Key: "_id", Value: 1}}))
}

// EarliestInWindow returns the first entry (by _id) with one of the keys in w, or nil.
func (s *Store) EarliestInWindow(ctx context.Context, game, userID string, keys []string, w Window) (*LogEntry, error) {
	f := base(game, userID)
	f["eventKey"] = bson.M{"$in": keys}
	w.apply(f)
	return s.findOne(ctx, f, bson.D{{Key: "_id", Value: 1}})
}

// LatestInWindow returns the last entry (by _id) with one of the keys in w, or nil.
func (s *Store) LatestInWindow(ctx context.Context, game, userID string, keys []string, w Window) (*LogEntry, error) {
	f := base(game, userID)
	f["eventKey"] = bson.M{"$in": keys}
	w.apply(f)
	return s.findOne(ctx, f, bson.D{{Key: "_id", Value: -1}})
}

func typeFilter(game, userID, eventType string, data map[string]any, w Window) bson.M {
	f := base(game, userID)
	f["eventType"] = eventType
	for k, v := range data {
		f["data."+k] = v
	}
	w.apply(f)
	return f
}

// ExistsByEventTypeAndDataInWindow reports whether an entry of eventType
// whose data fields satisfy data lies in w.
func (s *Store) ExistsByEventTypeAndDataInWindow(ctx context.Context, game, userID, eventType string, data map[string]any, w Window) (bool, error) {
	n, err := s.coll.CountDocuments(ctx, typeFilter(game, userID, eventType, data, w), options.Count().SetLimit(1))
	return n > 0, err
}

// CountByEventTypeAndDataInWindow counts entries of eventType whose data
// fields satisfy data in w.
func (s *Store) CountByEventTypeAndDataInWindow(ctx context.Context, game, userID, eventType string, data map[string]any, w Window) (int64, error) {
	return s.coll.CountDocuments(ctx, typeFilter(game, userID, eventType, data, w))
}

// EarliestByEventTypeAndDataInWindow returns the first matching entry in w, or nil.
func (s *Store) EarliestByEventTypeAndDataInWindow(ctx context.Context, game, userID, eventType string, data map[string]any, w Window) (*LogEntry, error) {
	return s.findOne(ctx, typeFilter(game, userID, eventType, data, w), bson.D{{Key: "_id", Value: 1}})
}

// LatestByEventTypeAndDataInWindow returns the last matching entry in w, or nil.
func (s *Store) LatestByEventTypeAndDataInWindow(ctx context.Context, game, userID, eventType string, data map[string]any, w Window) (*LogEntry, error) {
	return s.findOne(ctx, typeFilter(game, userID, eventType, data, w), bson.D{{Key: "_id", Value: -1}})
}

// FindByEventTypeAndDataInWindow returns matching entries in w, ascending by _id.
func (s *Store) FindByEventTypeAndDataInWindow(ctx context.Context, game, userID, eventType string, data map[string]any, w Window) ([]LogEntry, error) {
	return s.find(ctx, typeFilter(game, userID, eventType, data, w), options.Find().SetSort(bson.D{{Key: "_id", Value: 1}}))
}

// FindAllInIDWindow returns every entry for the user with _id in [startID, endID],
// ascending, projected to _id and serverTimestamp. Used for active duration.
func (s *Store) FindAllInIDWindow(ctx context.Context, game, userID string, startID, endID primitive.ObjectID) ([]LogEntry, error) {
	f := base(game, userID)
	f["_id"] = bson.M{"$gte": startID, "$lte": endID}
	opts := options.Find().
		SetSort(bson.D{{Key: "_id", Value: 1}}).
		SetProjection(bson.M{"_id": 1, "serverTimestamp": 1})
	return s.find(ctx, f, opts)
}

func (s *Store) find(ctx context.Context, filter bson.M, opts *options.FindOptions) ([]LogEntry, error) {
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

func (s *Store) findOne(ctx context.Context, filter bson.M, sort bson.D) (*LogEntry, error) {
	var e LogEntry
	err := s.coll.FindOne(ctx, filter, options.FindOne().SetSort(sort)).Decode(&e)
	if err == mongo.ErrNoDocuments {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &e, nil
}
