// internal/app/grader/scanner.go
package grader

import (
	"context"

	"github.com/dalemusser/mhsgrader/internal/app/store/graderstate"
	"github.com/dalemusser/mhsgrader/internal/app/store/logdata"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.uber.org/zap"
)

// Scanner scans logdata for trigger events.
type Scanner struct {
	logStore   *logdata.Store
	stateStore *graderstate.Store
	graderID   string
	game       string
	batchSize  int
	logger     *zap.Logger
}

// NewScanner creates a new scanner.
// logDB is used for reading logs (stratalog), gradesDB is used for storing cursor state (mhsgrader).
func NewScanner(logDB, gradesDB *mongo.Database, logger *zap.Logger, graderID, game string, batchSize int) *Scanner {
	return &Scanner{
		logStore:   logdata.New(logDB),
		stateStore: graderstate.New(gradesDB),
		graderID:   graderID,
		game:       game,
		batchSize:  batchSize,
		logger:     logger,
	}
}

// TriggerEvent represents a log event that triggered a rule evaluation.
type TriggerEvent struct {
	ID       primitive.ObjectID
	PlayerID string
	EventKey string
}

// Scan scans for new trigger events.
// Returns the events found and the last seen ID.
func (s *Scanner) Scan(ctx context.Context, triggerKeys []string) ([]TriggerEvent, primitive.ObjectID, error) {
	// Get current state
	state, err := s.stateStore.Get(ctx, s.graderID)
	if err != nil {
		return nil, primitive.NilObjectID, err
	}

	// Scan for new triggers
	entries, err := s.logStore.ScanTriggers(ctx, s.game, triggerKeys, state.LastSeenID, s.batchSize)
	if err != nil {
		return nil, primitive.NilObjectID, err
	}

	if len(entries) == 0 {
		return nil, state.LastSeenID, nil
	}

	// Convert to trigger events
	events := make([]TriggerEvent, len(entries))
	for i, entry := range entries {
		events[i] = TriggerEvent{
			ID:       entry.ID,
			PlayerID: entry.PlayerID,
			EventKey: entry.EventKey,
		}
	}

	// Return the last ID seen
	lastID := entries[len(entries)-1].ID

	s.logger.Debug("scanned for triggers",
		zap.Int("found", len(events)),
		zap.String("lastSeenId", lastID.Hex()),
	)

	return events, lastID, nil
}

// UpdateCursor updates the last seen ID in the state.
func (s *Scanner) UpdateCursor(ctx context.Context, lastSeenID primitive.ObjectID) error {
	return s.stateStore.UpdateLastSeenID(ctx, s.graderID, lastSeenID)
}

// Reset clears the scanner state to reprocess all logs.
func (s *Scanner) Reset(ctx context.Context) error {
	s.logger.Info("resetting scanner state for reprocessing")
	return s.stateStore.Reset(ctx, s.graderID)
}
