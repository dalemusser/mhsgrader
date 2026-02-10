// internal/app/rules/helpers.go
package rules

import (
	"context"

	"github.com/dalemusser/mhsgrader/internal/app/store/logdata"
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
