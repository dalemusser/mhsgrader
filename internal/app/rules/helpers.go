// internal/app/rules/helpers.go
package rules

import (
	"context"
	"fmt"
	"math"

	"github.com/dalemusser/mhsgrader/internal/app/store/logdata"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

// AttemptWindow is the graded window: (StartID, EndID] by _id, optionally
// fenced by the client timestamp as well (see WindowSpec.TimestampFence).
type AttemptWindow struct {
	StartID primitive.ObjectID // exclusive
	EndID   primitive.ObjectID // inclusive
	TSStart *bson.RawValue
	TSEnd   *bson.RawValue
}

// ZeroID returns the MongoDB zero ObjectID for unbounded start.
func ZeroID() primitive.ObjectID {
	return primitive.ObjectID{}
}

// Sub returns an _id-only sub-window (startID, endID] inside this window,
// e.g. a phase of the activity delimited by marker events.
func (w *AttemptWindow) Sub(startID, endID primitive.ObjectID) *AttemptWindow {
	return &AttemptWindow{StartID: startID, EndID: endID}
}

func (w *AttemptWindow) store() logdata.Window {
	return logdata.Window{StartID: w.StartID, EndID: w.EndID, TSStart: w.TSStart, TSEnd: w.TSEnd}
}

// UnitScene matches the `data.Unit` scene name of a unit (e.g. "Unit 4 Dev",
// "Unit 4 Prod") the way the spec does: by prefix.
func UnitScene(unit int) primitive.Regex {
	return primitive.Regex{Pattern: fmt.Sprintf("^Unit %d", unit)}
}

// JSRound mirrors JavaScript's Math.round (halves round toward +infinity),
// which the spec's message scripts use.
func JSRound(x float64) int64 {
	return int64(math.Floor(x + 0.5))
}

// LogDataHelper provides the query patterns rules need, all bounded to the
// graded window. A nil window yields empty results.
type LogDataHelper struct {
	store *logdata.Store
	game  string
}

// NewLogDataHelper creates a new helper.
func NewLogDataHelper(db *mongo.Database, game string) *LogDataHelper {
	return &LogDataHelper{store: logdata.New(db), game: game}
}

// ---- eventKey queries -------------------------------------------------------

// HasEventInWindow reports whether the eventKey occurs in the window.
func (h *LogDataHelper) HasEventInWindow(ctx context.Context, userID, eventKey string, w *AttemptWindow) (bool, error) {
	return h.HasAnyEventInWindow(ctx, userID, []string{eventKey}, w)
}

// HasAnyEventInWindow reports whether any of the eventKeys occurs in the window.
func (h *LogDataHelper) HasAnyEventInWindow(ctx context.Context, userID string, eventKeys []string, w *AttemptWindow) (bool, error) {
	if w == nil || len(eventKeys) == 0 {
		return false, nil
	}
	return h.store.ExistsInWindow(ctx, h.game, userID, eventKeys, w.store())
}

// CountEventInIDWindow counts occurrences of one eventKey in the window.
func (h *LogDataHelper) CountEventInIDWindow(ctx context.Context, userID, eventKey string, w *AttemptWindow) (int64, error) {
	return h.CountEventsInWindow(ctx, userID, []string{eventKey}, w)
}

// CountEventsInWindow counts occurrences of any of the eventKeys in the window.
func (h *LogDataHelper) CountEventsInWindow(ctx context.Context, userID string, eventKeys []string, w *AttemptWindow) (int64, error) {
	if w == nil || len(eventKeys) == 0 {
		return 0, nil
	}
	return h.store.CountInWindow(ctx, h.game, userID, eventKeys, w.store())
}

// FindEventsInWindow returns the entries with any of the eventKeys in the
// window, ascending by _id.
func (h *LogDataHelper) FindEventsInWindow(ctx context.Context, userID string, eventKeys []string, w *AttemptWindow) ([]logdata.LogEntry, error) {
	if w == nil || len(eventKeys) == 0 {
		return nil, nil
	}
	return h.store.FindInWindow(ctx, h.game, userID, eventKeys, w.store())
}

// EarliestEventInWindow returns the first entry with any of the eventKeys
// in the window, or nil.
func (h *LogDataHelper) EarliestEventInWindow(ctx context.Context, userID string, eventKeys []string, w *AttemptWindow) (*logdata.LogEntry, error) {
	if w == nil || len(eventKeys) == 0 {
		return nil, nil
	}
	return h.store.EarliestInWindow(ctx, h.game, userID, eventKeys, w.store())
}

// LatestEventInWindow returns the last entry with any of the eventKeys in
// the window, or nil.
func (h *LogDataHelper) LatestEventInWindow(ctx context.Context, userID string, eventKeys []string, w *AttemptWindow) (*logdata.LogEntry, error) {
	if w == nil || len(eventKeys) == 0 {
		return nil, nil
	}
	return h.store.LatestInWindow(ctx, h.game, userID, eventKeys, w.store())
}

// ---- eventType + data queries ---------------------------------------------

// HasEventTypeWithDataInWindow reports whether an entry of eventType with
// data[dataField] == dataValue occurs in the window.
func (h *LogDataHelper) HasEventTypeWithDataInWindow(ctx context.Context, userID, eventType, dataField, dataValue string, w *AttemptWindow) (bool, error) {
	return h.HasEventTypeAndData(ctx, userID, eventType, map[string]any{dataField: dataValue}, w)
}

// HasEventTypeAndData reports whether an entry of eventType whose data fields
// satisfy the filter occurs in the window. Values may be strings or
// primitive.Regex (see UnitScene).
func (h *LogDataHelper) HasEventTypeAndData(ctx context.Context, userID, eventType string, dataFilter map[string]any, w *AttemptWindow) (bool, error) {
	if w == nil {
		return false, nil
	}
	return h.store.ExistsByEventTypeAndDataInWindow(ctx, h.game, userID, eventType, dataFilter, w.store())
}

// CountByEventTypeAndData counts entries of eventType whose data fields
// satisfy the filter in the window.
func (h *LogDataHelper) CountByEventTypeAndData(ctx context.Context, userID, eventType string, dataFilter map[string]any, w *AttemptWindow) (int64, error) {
	if w == nil {
		return 0, nil
	}
	return h.store.CountByEventTypeAndDataInWindow(ctx, h.game, userID, eventType, dataFilter, w.store())
}

// FindByEventTypeAndData returns matching entries in the window, ascending by _id.
func (h *LogDataHelper) FindByEventTypeAndData(ctx context.Context, userID, eventType string, dataFilter map[string]any, w *AttemptWindow) ([]logdata.LogEntry, error) {
	if w == nil {
		return nil, nil
	}
	return h.store.FindByEventTypeAndDataInWindow(ctx, h.game, userID, eventType, dataFilter, w.store())
}

// FindLatestByEventTypeAndData returns the most recent matching entry in the window, or nil.
func (h *LogDataHelper) FindLatestByEventTypeAndData(ctx context.Context, userID, eventType string, dataFilter map[string]any, w *AttemptWindow) (*logdata.LogEntry, error) {
	if w == nil {
		return nil, nil
	}
	return h.store.LatestByEventTypeAndDataInWindow(ctx, h.game, userID, eventType, dataFilter, w.store())
}

// FindEarliestByEventTypeAndData returns the first matching entry in the window, or nil.
func (h *LogDataHelper) FindEarliestByEventTypeAndData(ctx context.Context, userID, eventType string, dataFilter map[string]any, w *AttemptWindow) (*logdata.LogEntry, error) {
	if w == nil {
		return nil, nil
	}
	return h.store.EarliestByEventTypeAndDataInWindow(ctx, h.game, userID, eventType, dataFilter, w.store())
}

// FindEventPairByEventTypeAndData returns the earliest entry matching
// firstData and the latest entry matching lastData in the window (either
// may be nil). Used for timing calculations.
func (h *LogDataHelper) FindEventPairByEventTypeAndData(ctx context.Context, userID, eventType string, firstData, lastData map[string]any, w *AttemptWindow) (*logdata.LogEntry, *logdata.LogEntry, error) {
	if w == nil {
		return nil, nil, nil
	}
	first, err := h.store.EarliestByEventTypeAndDataInWindow(ctx, h.game, userID, eventType, firstData, w.store())
	if err != nil {
		return nil, nil, err
	}
	last, err := h.store.LatestByEventTypeAndDataInWindow(ctx, h.game, userID, eventType, lastData, w.store())
	if err != nil {
		return nil, nil, err
	}
	return first, last, nil
}
