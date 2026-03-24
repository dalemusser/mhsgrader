// internal/app/grader/evaluator.go
package grader

import (
	"context"
	"errors"
	"time"

	"github.com/dalemusser/mhsgrader/internal/app/rules"
	"github.com/dalemusser/mhsgrader/internal/app/store/logdata"
	"github.com/dalemusser/mhsgrader/internal/app/store/progressgrades"
	"go.mongodb.org/mongo-driver/mongo"
	"go.uber.org/zap"
)

// Evaluator evaluates rules and stores grades.
type Evaluator struct {
	registry           *rules.Registry
	gradeStore         *progressgrades.Store
	logStore           *logdata.Store
	logDB              *mongo.Database
	game               string
	activeGapThreshold time.Duration
	logger             *zap.Logger
}

// NewEvaluator creates a new evaluator.
// logDB is used for rule evaluation queries (stratalog), gradesDB is used for storing grades (mhsgrader).
func NewEvaluator(logDB, gradesDB *mongo.Database, registry *rules.Registry, logger *zap.Logger, game string, activeGapThreshold time.Duration) *Evaluator {
	return &Evaluator{
		registry:           registry,
		gradeStore:         progressgrades.New(gradesDB),
		logStore:           logdata.New(logDB),
		logDB:              logDB,
		game:               game,
		activeGapThreshold: activeGapThreshold,
		logger:             logger,
	}
}

// EvaluateAndStore processes a scanned event: handles unit starts, point starts (active), and end triggers (evaluate).
// Processes all rules for the event (so one failing rule doesn't block others),
// but returns an error if any rule failed so the cursor won't advance past this event.
// On retry, already-stored grades are safely overwritten (UpsertGrade is idempotent).
func (e *Evaluator) EvaluateAndStore(ctx context.Context, event TriggerEvent) error {
	var firstErr error

	// Check if this is a unit start event
	if unitID := e.registry.GetUnitForStartKey(event.EventKey); unitID != "" {
		if err := e.gradeStore.SetCurrentUnit(ctx, e.game, event.PlayerID, unitID); err != nil {
			e.logger.Error("failed to set current unit",
				zap.String("unitId", unitID),
				zap.String("playerId", event.PlayerID),
				zap.Error(err),
			)
			firstErr = errors.Join(firstErr, err)
		} else {
			e.logger.Debug("current unit updated",
				zap.String("unitId", unitID),
				zap.String("playerId", event.PlayerID),
			)
		}
	}

	// Handle start triggers — set "active" status
	startRules := e.registry.GetStartRulesForKey(event.EventKey)
	for _, rule := range startRules {
		startTime := event.ServerTimestamp
		if err := e.gradeStore.AppendActiveIfNeeded(ctx, e.game, event.PlayerID, rule.PointID(), rule.ID(), &startTime); err != nil {
			e.logger.Error("failed to set active status",
				zap.String("rule", rule.ID()),
				zap.String("playerId", event.PlayerID),
				zap.Error(err),
			)
			firstErr = errors.Join(firstErr, err)
			continue
		}
		e.logger.Debug("active status set",
			zap.String("rule", rule.ID()),
			zap.String("playerId", event.PlayerID),
			zap.String("pointId", rule.PointID()),
		)
	}

	// Handle end triggers — evaluate rules and store passed/flagged
	endRules := e.registry.GetEndRulesForKey(event.EventKey)
	for _, rule := range endRules {
		// Build EvalContext: find start event and construct window
		ec := rules.EvalContext{
			EndTime:    event.ServerTimestamp,
			EndEventID: event.ID,
		}

		startKeys := rule.StartKeys()
		var startEntry *logdata.LogEntry
		if len(startKeys) > 0 {
			var err error
			startEntry, err = e.logStore.GetLatestByEventKeysBefore(ctx, e.game, event.PlayerID, startKeys, event.ID)
			if err != nil {
				e.logger.Warn("failed to look up start event for EvalContext",
					zap.String("rule", rule.ID()),
					zap.String("playerId", event.PlayerID),
					zap.Error(err),
				)
			}
		}

		if startEntry != nil {
			ec.StartTime = &startEntry.ServerTimestamp
			ec.Window = &rules.AttemptWindow{
				StartID: startEntry.ID,
				EndID:   event.ID,
			}
		} else {
			ec.Window = &rules.AttemptWindow{
				StartID: rules.ZeroID(),
				EndID:   event.ID,
			}
		}

		result, err := rule.Evaluate(ctx, e.logDB, e.game, event.PlayerID, ec)
		if err != nil {
			e.logger.Error("rule evaluation failed",
				zap.String("rule", rule.ID()),
				zap.String("playerId", event.PlayerID),
				zap.Error(err),
			)
			firstErr = errors.Join(firstErr, err)
			continue
		}

		endTime := event.ServerTimestamp
		grade := progressgrades.Grade{
			Status:     result.Status,
			RuleID:     rule.ID(),
			ReasonCode: result.ReasonCode,
			Metrics:    result.Metrics,
			StartTime:  ec.StartTime,
			EndTime:    &endTime,
		}

		// Calculate durations reusing the start entry from EvalContext
		grade.DurationSecs = e.calcDurationFromStart(startEntry, event)
		grade.ActiveDurationSecs = e.calcActiveDurationFromStart(ctx, startEntry, event, rule)

		if err := e.gradeStore.AppendGrade(ctx, e.game, event.PlayerID, rule.PointID(), grade); err != nil {
			e.logger.Error("failed to store grade",
				zap.String("rule", rule.ID()),
				zap.String("playerId", event.PlayerID),
				zap.Error(err),
			)
			firstErr = errors.Join(firstErr, err)
			continue
		}

		e.logger.Debug("grade stored",
			zap.String("rule", rule.ID()),
			zap.String("playerId", event.PlayerID),
			zap.String("pointId", rule.PointID()),
			zap.String("status", result.Status),
		)
	}

	return firstErr
}

// calcDurationFromStart computes wall-clock duration using a pre-fetched start entry.
// Returns nil if start entry is nil or duration is negative.
func (e *Evaluator) calcDurationFromStart(startEntry *logdata.LogEntry, endEvent TriggerEvent) *float64 {
	if startEntry == nil {
		return nil
	}

	duration := endEvent.ServerTimestamp.Sub(startEntry.ServerTimestamp).Seconds()
	if duration < 0 {
		return nil
	}

	return &duration
}

// calcActiveDurationFromStart computes active duration using a pre-fetched start entry.
// Sums only gaps between consecutive log entries shorter than activeGapThreshold.
func (e *Evaluator) calcActiveDurationFromStart(ctx context.Context, startEntry *logdata.LogEntry, endEvent TriggerEvent, rule rules.Rule) *float64 {
	if startEntry == nil {
		return nil
	}

	// Get all log entries for this player between start and end events
	entries, err := e.logStore.FindAllInIDWindow(ctx, e.game, endEvent.PlayerID, startEntry.ID, endEvent.ID)
	if err != nil {
		e.logger.Warn("failed to fetch log entries for active duration",
			zap.String("rule", rule.ID()),
			zap.String("playerId", endEvent.PlayerID),
			zap.Error(err),
		)
		return nil
	}

	if len(entries) < 2 {
		return nil
	}

	// Walk entries chronologically, summing gaps under the threshold
	var activeSecs float64
	for i := 1; i < len(entries); i++ {
		gap := entries[i].ServerTimestamp.Sub(entries[i-1].ServerTimestamp)
		if gap > 0 && gap <= e.activeGapThreshold {
			activeSecs += gap.Seconds()
		}
	}

	return &activeSecs
}
