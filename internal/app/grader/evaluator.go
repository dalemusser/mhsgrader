// internal/app/grader/evaluator.go
package grader

import (
	"context"

	"github.com/dalemusser/mhsgrader/internal/app/rules"
	"github.com/dalemusser/mhsgrader/internal/app/store/progressgrades"
	"go.mongodb.org/mongo-driver/mongo"
	"go.uber.org/zap"
)

// Evaluator evaluates rules and stores grades.
type Evaluator struct {
	registry   *rules.Registry
	gradeStore *progressgrades.Store
	logDB      *mongo.Database
	game       string
	logger     *zap.Logger
}

// NewEvaluator creates a new evaluator.
// logDB is used for rule evaluation queries (stratalog), gradesDB is used for storing grades (mhsgrader).
func NewEvaluator(logDB, gradesDB *mongo.Database, registry *rules.Registry, logger *zap.Logger, game string) *Evaluator {
	return &Evaluator{
		registry:   registry,
		gradeStore: progressgrades.New(gradesDB),
		logDB:      logDB,
		game:       game,
		logger:     logger,
	}
}

// EvaluateAndStore processes a scanned event: handles unit starts, point starts (active), and end triggers (evaluate).
func (e *Evaluator) EvaluateAndStore(ctx context.Context, event TriggerEvent) error {
	// Check if this is a unit start event
	if unitID := e.registry.GetUnitForStartKey(event.EventKey); unitID != "" {
		if err := e.gradeStore.SetCurrentUnit(ctx, e.game, event.PlayerID, unitID); err != nil {
			e.logger.Error("failed to set current unit",
				zap.String("unitId", unitID),
				zap.String("playerId", event.PlayerID),
				zap.Error(err),
			)
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
		if err := e.gradeStore.SetActiveIfPending(ctx, e.game, event.PlayerID, rule.PointID(), rule.ID()); err != nil {
			e.logger.Error("failed to set active status",
				zap.String("rule", rule.ID()),
				zap.String("playerId", event.PlayerID),
				zap.Error(err),
			)
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
		result, err := rule.Evaluate(ctx, e.logDB, e.game, event.PlayerID)
		if err != nil {
			e.logger.Error("rule evaluation failed",
				zap.String("rule", rule.ID()),
				zap.String("playerId", event.PlayerID),
				zap.Error(err),
			)
			continue
		}

		grade := progressgrades.Grade{
			Status:     result.Status,
			RuleID:     rule.ID(),
			ReasonCode: result.ReasonCode,
			Metrics:    result.Metrics,
		}

		if err := e.gradeStore.UpsertGrade(ctx, e.game, event.PlayerID, rule.PointID(), grade); err != nil {
			e.logger.Error("failed to store grade",
				zap.String("rule", rule.ID()),
				zap.String("playerId", event.PlayerID),
				zap.Error(err),
			)
			continue
		}

		e.logger.Debug("grade stored",
			zap.String("rule", rule.ID()),
			zap.String("playerId", event.PlayerID),
			zap.String("pointId", rule.PointID()),
			zap.String("status", result.Status),
		)
	}

	return nil
}
