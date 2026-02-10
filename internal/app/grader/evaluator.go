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

// EvaluateAndStore evaluates rules for a trigger event and stores the result.
func (e *Evaluator) EvaluateAndStore(ctx context.Context, event TriggerEvent) error {
	// Get rules for this event key
	rulesToEval := e.registry.GetRulesForKey(event.EventKey)
	if len(rulesToEval) == 0 {
		return nil
	}

	for _, rule := range rulesToEval {
		result, err := rule.Evaluate(ctx, e.logDB, e.game, event.PlayerID)
		if err != nil {
			e.logger.Error("rule evaluation failed",
				zap.String("rule", rule.ID()),
				zap.String("playerId", event.PlayerID),
				zap.Error(err),
			)
			continue // Continue with other rules
		}

		// Store the grade
		grade := progressgrades.Grade{
			Color:      result.Color,
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
			zap.String("color", result.Color),
		)
	}

	return nil
}
