// internal/app/grader/engine.go
package grader

import (
	"context"
	"time"

	"github.com/dalemusser/mhsgrader/internal/app/rules"
	"go.mongodb.org/mongo-driver/mongo"
	"go.uber.org/zap"
)

// Engine is the main grading engine that coordinates scanning and evaluation.
type Engine struct {
	scanner      *Scanner
	evaluator    *Evaluator
	registry     *rules.Registry
	scanInterval time.Duration
	reprocessAll bool
	logger       *zap.Logger
}

// NewEngine creates a new grading engine.
// logDB is used for reading logs (stratalog), gradesDB is used for storing grades (mhsgrader).
func NewEngine(logDB, gradesDB *mongo.Database, logger *zap.Logger, game string, scanInterval time.Duration, batchSize int, reprocessAll bool) *Engine {
	registry := rules.DefaultRegistry()
	graderID := game + "-grader"

	return &Engine{
		scanner:      NewScanner(logDB, gradesDB, logger, graderID, game, batchSize),
		evaluator:    NewEvaluator(logDB, gradesDB, registry, logger, game),
		registry:     registry,
		scanInterval: scanInterval,
		reprocessAll: reprocessAll,
		logger:       logger,
	}
}

// Run starts the grading engine and blocks until the context is cancelled.
func (e *Engine) Run(ctx context.Context) error {
	// If reprocess_all is set, reset the cursor to start from the beginning
	if e.reprocessAll {
		if err := e.scanner.Reset(ctx); err != nil {
			return err
		}
	}

	// Get all trigger keys we need to watch
	triggerKeys := e.registry.AllTriggerKeys()
	e.logger.Info("starting grading engine",
		zap.Int("triggerKeys", len(triggerKeys)),
		zap.Duration("scanInterval", e.scanInterval),
		zap.Bool("reprocessAll", e.reprocessAll),
	)

	ticker := time.NewTicker(e.scanInterval)
	defer ticker.Stop()

	// Run immediately on start
	if err := e.tick(ctx, triggerKeys); err != nil {
		e.logger.Error("initial tick failed", zap.Error(err))
	}

	for {
		select {
		case <-ctx.Done():
			e.logger.Info("grading engine stopping")
			return ctx.Err()
		case <-ticker.C:
			if err := e.tick(ctx, triggerKeys); err != nil {
				e.logger.Error("tick failed", zap.Error(err))
			}
		}
	}
}

// tick performs one scan-evaluate cycle.
func (e *Engine) tick(ctx context.Context, triggerKeys []string) error {
	// Scan for new triggers
	events, lastID, err := e.scanner.Scan(ctx, triggerKeys)
	if err != nil {
		return err
	}

	if len(events) == 0 {
		return nil
	}

	e.logger.Info("processing triggers",
		zap.Int("count", len(events)),
	)

	// Evaluate each trigger
	for _, event := range events {
		if err := e.evaluator.EvaluateAndStore(ctx, event); err != nil {
			e.logger.Error("evaluation failed",
				zap.String("eventKey", event.EventKey),
				zap.String("playerId", event.PlayerID),
				zap.Error(err),
			)
			// Continue with other events
		}
	}

	// Update cursor after processing
	if err := e.scanner.UpdateCursor(ctx, lastID); err != nil {
		e.logger.Error("failed to update cursor", zap.Error(err))
		return err
	}

	return nil
}
