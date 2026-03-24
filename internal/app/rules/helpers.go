// internal/app/rules/helpers.go
package rules

import (
	"context"

	"github.com/dalemusser/mhsgrader/internal/app/store/logdata"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

// LogDataHelper provides common query patterns for rules.
type LogDataHelper struct {
	store *logdata.Store
	game  string
}

// NewLogDataHelper creates a new helper.
func NewLogDataHelper(db *mongo.Database, game string) *LogDataHelper {
	return &LogDataHelper{
		store: logdata.New(db),
		game:  game,
	}
}

// HasEvent checks if the player has any log with the given eventKey.
func (h *LogDataHelper) HasEvent(ctx context.Context, playerID, eventKey string) (bool, error) {
	return h.store.ExistsByEventKey(ctx, h.game, playerID, eventKey)
}

// HasAnyEvent checks if the player has any log with any of the given eventKeys.
func (h *LogDataHelper) HasAnyEvent(ctx context.Context, playerID string, eventKeys []string) (bool, error) {
	return h.store.ExistsByEventKeys(ctx, h.game, playerID, eventKeys)
}

// CountEvent counts logs with the given eventKey.
func (h *LogDataHelper) CountEvent(ctx context.Context, playerID, eventKey string) (int64, error) {
	return h.store.CountByEventKey(ctx, h.game, playerID, eventKey)
}

// CountEventInWindow counts logs in a window starting from a specific event.
func (h *LogDataHelper) CountEventInWindow(ctx context.Context, playerID, windowStartEventKey, countEventKey string) (int64, error) {
	windowStart, err := h.store.GetWindowStart(ctx, h.game, playerID, windowStartEventKey)
	if err != nil || windowStart == nil {
		return 0, err
	}
	return h.store.CountByEventKeyInWindow(ctx, h.game, playerID, countEventKey, *windowStart)
}

// FindEvents finds all logs with the given eventKey.
func (h *LogDataHelper) FindEvents(ctx context.Context, playerID, eventKey string) ([]logdata.LogEntry, error) {
	return h.store.FindByEventKey(ctx, h.game, playerID, eventKey)
}

// ============================================================================
// AttemptWindow - _id-based attempt windowing for replay-safe grading
// ============================================================================

// AttemptWindow represents an _id-based attempt window for replay-safe grading.
// The window is (StartID, EndID] - exclusive on start, inclusive on end.
type AttemptWindow struct {
	StartID primitive.ObjectID // Exclusive (previous trigger)
	EndID   primitive.ObjectID // Inclusive (current trigger)
}

// ZeroID returns the MongoDB zero ObjectID for unbounded start.
func ZeroID() primitive.ObjectID {
	return primitive.ObjectID{}
}

// HasEventInWindow checks if event exists within attempt window.
func (h *LogDataHelper) HasEventInWindow(ctx context.Context, playerID, eventKey string, w *AttemptWindow) (bool, error) {
	if w == nil {
		return false, nil
	}
	return h.store.ExistsByEventKeyInIDWindow(ctx, h.game, playerID, eventKey, w.StartID, w.EndID)
}

// HasAnyEventInWindow checks if any event exists within attempt window.
func (h *LogDataHelper) HasAnyEventInWindow(ctx context.Context, playerID string, eventKeys []string, w *AttemptWindow) (bool, error) {
	if w == nil {
		return false, nil
	}
	return h.store.ExistsByEventKeysInIDWindow(ctx, h.game, playerID, eventKeys, w.StartID, w.EndID)
}

// CountEventInIDWindow counts events within attempt window.
func (h *LogDataHelper) CountEventInIDWindow(ctx context.Context, playerID, eventKey string, w *AttemptWindow) (int64, error) {
	if w == nil {
		return 0, nil
	}
	return h.store.CountByEventKeyInIDWindow(ctx, h.game, playerID, eventKey, w.StartID, w.EndID)
}

// CountEventsInWindow counts events matching any key within attempt window.
func (h *LogDataHelper) CountEventsInWindow(ctx context.Context, playerID string, eventKeys []string, w *AttemptWindow) (int64, error) {
	if w == nil {
		return 0, nil
	}
	return h.store.CountByEventKeysInIDWindow(ctx, h.game, playerID, eventKeys, w.StartID, w.EndID)
}

// HasEventTypeWithDataInWindow checks for eventType + data match in window.
func (h *LogDataHelper) HasEventTypeWithDataInWindow(ctx context.Context, playerID, eventType, dataField, dataValue string, w *AttemptWindow) (bool, error) {
	if w == nil {
		return false, nil
	}
	return h.store.ExistsByEventTypeAndDataInIDWindow(ctx, h.game, playerID, eventType, dataField, dataValue, w.StartID, w.EndID)
}

// CountByEventTypeAndData counts events matching eventType + data fields in window.
func (h *LogDataHelper) CountByEventTypeAndData(ctx context.Context, playerID, eventType string, dataFilter map[string]string, w *AttemptWindow) (int64, error) {
	if w == nil {
		return 0, nil
	}
	return h.store.CountByEventTypeAndDataInIDWindow(ctx, h.game, playerID, eventType, dataFilter, w.StartID, w.EndID)
}

// FindLatestByEventTypeAndData finds the most recent event matching eventType + data in window.
func (h *LogDataHelper) FindLatestByEventTypeAndData(ctx context.Context, playerID, eventType string, dataFilter map[string]string, w *AttemptWindow) (*logdata.LogEntry, error) {
	if w == nil {
		return nil, nil
	}
	return h.store.FindLatestByEventTypeAndDataInIDWindow(ctx, h.game, playerID, eventType, dataFilter, w.StartID, w.EndID)
}

// FindEventPairByEventTypeAndData finds first and last events matching eventType + data in window.
// Used for timing calculations (e.g., duration between start and end of a puzzle).
func (h *LogDataHelper) FindEventPairByEventTypeAndData(ctx context.Context, playerID, eventType string, firstData, lastData map[string]string, w *AttemptWindow) (*logdata.LogEntry, *logdata.LogEntry, error) {
	if w == nil {
		return nil, nil, nil
	}
	return h.store.FindEventPairByEventTypeAndDataInIDWindow(ctx, h.game, playerID, eventType, firstData, lastData, w.StartID, w.EndID)
}
