// internal/app/grader/evaluator.go
package grader

import (
	"bytes"
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
// On retry, already-stored grades are safely overwritten (the grade store is idempotent per attempt).
func (e *Evaluator) EvaluateAndStore(ctx context.Context, event TriggerEvent) error {
	var firstErr error

	// Check if this is a unit start event
	if unitID := e.registry.GetUnitForStartKey(event.EventKey); unitID != "" {
		if err := e.gradeStore.SetCurrentUnit(ctx, e.game, event.UserID, unitID); err != nil {
			e.logger.Error("failed to set current unit",
				zap.String("unitId", unitID),
				zap.String("user_id", event.UserID),
				zap.Error(err),
			)
			firstErr = errors.Join(firstErr, err)
		} else {
			e.logger.Debug("current unit updated",
				zap.String("unitId", unitID),
				zap.String("user_id", event.UserID),
			)
		}
	}

	// Handle start anchors — set "active" status
	for _, rule := range e.registry.GetStartRulesForEvent(&event) {
		startTime := event.ServerTimestamp
		if err := e.gradeStore.AppendActiveIfNeeded(ctx, e.game, event.UserID, rule.PointID(), rule.ID(), &startTime); err != nil {
			e.logger.Error("failed to set active status",
				zap.String("rule", rule.ID()),
				zap.String("user_id", event.UserID),
				zap.Error(err),
			)
			firstErr = errors.Join(firstErr, err)
			continue
		}
		e.logger.Debug("active status set",
			zap.String("rule", rule.ID()),
			zap.String("user_id", event.UserID),
			zap.String("pointId", rule.PointID()),
		)
	}

	// Handle end triggers — evaluate rules and store passed/flagged
	for _, rule := range e.registry.GetEndRulesForEvent(&event) {
		ec, startEntry, skip, err := e.buildContext(ctx, rule, event)
		if err != nil {
			e.logger.Error("failed to build the graded window",
				zap.String("rule", rule.ID()),
				zap.String("user_id", event.UserID),
				zap.Error(err),
			)
			firstErr = errors.Join(firstErr, err)
			continue
		}
		if skip {
			e.logger.Debug("trigger re-fired with no new start; ignored",
				zap.String("rule", rule.ID()),
				zap.String("user_id", event.UserID),
				zap.String("eventId", event.ID.Hex()),
			)
			continue
		}

		result, err := rule.Evaluate(ctx, e.logDB, e.game, event.UserID, ec)
		if err != nil {
			e.logger.Error("rule evaluation failed",
				zap.String("rule", rule.ID()),
				zap.String("user_id", event.UserID),
				zap.Error(err),
			)
			firstErr = errors.Join(firstErr, err)
			continue
		}

		metrics := result.Metrics
		if ec.WindowNote != "" {
			if metrics == nil {
				metrics = map[string]any{}
			}
			metrics["window"] = ec.WindowNote
		}

		endTime := event.ServerTimestamp
		grade := progressgrades.Grade{
			Status:     result.Status,
			RuleID:     rule.ID(),
			ReasonCode: result.ReasonCode,
			Reasons:    toStoredReasons(result.Reasons),
			Metrics:    metrics,
			StartTime:  ec.StartTime,
			EndTime:    &endTime,
		}

		// Durations are measured from the start anchor (not the graded window)
		grade.DurationSecs = e.calcDurationFromStart(startEntry, event)
		grade.ActiveDurationSecs = e.calcActiveDurationFromStart(ctx, startEntry, event, rule)

		if err := e.gradeStore.AppendGrade(ctx, e.game, event.UserID, rule.PointID(), grade); err != nil {
			e.logger.Error("failed to store grade",
				zap.String("rule", rule.ID()),
				zap.String("user_id", event.UserID),
				zap.Error(err),
			)
			firstErr = errors.Join(firstErr, err)
			continue
		}

		e.logger.Debug("grade stored",
			zap.String("rule", rule.ID()),
			zap.String("user_id", event.UserID),
			zap.String("pointId", rule.PointID()),
			zap.String("status", result.Status),
		)
	}

	return firstErr
}

// buildContext resolves the start anchor and the graded window for a trigger
// according to the rule's WindowSpec. skip is true when the trigger is a
// re-fire that must not produce a new attempt.
func (e *Evaluator) buildContext(ctx context.Context, rule rules.Rule, event TriggerEvent) (ec rules.EvalContext, startEntry *logdata.LogEntry, skip bool, err error) {
	ec = rules.EvalContext{EndTime: event.ServerTimestamp, EndEventID: event.ID}

	hasStartAnchors := len(rule.StartKeys())+len(rule.StartMatchers()) > 0
	if hasStartAnchors {
		startEntry, err = e.logStore.LatestBefore(ctx, e.game, event.UserID, rule.StartKeys(), rule.StartMatchers(), event.ID)
		if err != nil {
			return ec, nil, false, err
		}
	}
	if startEntry != nil {
		ec.StartTime = &startEntry.ServerTimestamp
		ec.StartEventID = startEntry.ID
	}

	w := &rules.AttemptWindow{StartID: rules.ZeroID(), EndID: event.ID}
	spec := rule.Window()
	switch spec.Kind {
	case rules.WindowPrevTrigger:
		prev, perr := e.logStore.LatestBefore(ctx, e.game, event.UserID, rule.TriggerKeys(), rule.TriggerMatchers(), event.ID)
		if perr != nil {
			return ec, startEntry, false, perr
		}
		if prev != nil {
			if hasStartAnchors && (startEntry == nil || bytes.Compare(startEntry.ID[:], prev.ID[:]) <= 0) {
				return ec, startEntry, true, nil
			}
			w.StartID = prev.ID
		}
	default: // WindowStartBeforeEnd
		if startEntry != nil {
			w.StartID = startEntry.ID
			if spec.TimestampFence && startEntry.HasTimestamp() && event.HasTimestamp() {
				ts, te := startEntry.Timestamp, event.Timestamp
				w.TSStart, w.TSEnd = &ts, &te
			}
		} else if hasStartAnchors {
			ec.WindowNote = "no start event found; graded from the beginning of the log"
		}
	}
	ec.Window = w
	return ec, startEntry, false, nil
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
	entries, err := e.logStore.FindAllInIDWindow(ctx, e.game, endEvent.UserID, startEntry.ID, endEvent.ID)
	if err != nil {
		e.logger.Warn("failed to fetch log entries for active duration",
			zap.String("rule", rule.ID()),
			zap.String("user_id", endEvent.UserID),
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

// toStoredReasons converts rule reasons to their stored form.
func toStoredReasons(rs []rules.Reason) []progressgrades.Reason {
	if len(rs) == 0 {
		return nil
	}
	out := make([]progressgrades.Reason, len(rs))
	for i, r := range rs {
		out[i] = progressgrades.Reason{Code: r.Code, Variables: r.Variables}
	}
	return out
}
