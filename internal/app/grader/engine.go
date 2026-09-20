// internal/app/grader/engine.go
package grader

import (
	"context"
	"time"

	"github.com/dalemusser/mhsgrader/internal/app/rules"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.uber.org/zap"
)

// Engine is the main grading engine that coordinates scanning and evaluation.
type Engine struct {
	scanner      *Scanner
	evaluator    *Evaluator
	registry     *rules.Registry
	scanInterval time.Duration
	logger       *zap.Logger
}

// NewEngine creates a new grading engine.
// logDB is used for reading logs (stratalog), gradesDB is used for storing grades (mhsgrader).
func NewEngine(logDB, gradesDB *mongo.Database, logger *zap.Logger, game string, scanInterval time.Duration, batchSize int, activeGapThreshold time.Duration) *Engine {
	registry := rules.DefaultRegistry()
	graderID := game + "-grader"

	return &Engine{
		scanner:      NewScanner(logDB, gradesDB, logger, graderID, game, batchSize),
		evaluator:    NewEvaluator(logDB, gradesDB, registry, logger, game, activeGapThreshold),
		registry:     registry,
		scanInterval: scanInterval,
		logger:       logger,
	}
}

// Run starts the grading engine and blocks until the context is cancelled.
func (e *Engine) Run(ctx context.Context) error {
	// Get all trigger keys we need to watch
	triggerKeys := e.registry.AllTriggerKeys()
	e.logger.Info("starting grading engine",
		zap.Int("triggerKeys", len(triggerKeys)),
		zap.Int("triggerMatchers", len(e.registry.AllTriggerMatchers())),
		zap.Duration("scanInterval", e.scanInterval),
	)

	ticker := time.NewTicker(e.scanInterval)
	defer ticker.Stop()

	// Run immediately on start
	if _, err := e.tick(ctx, triggerKeys); err != nil {
		e.logger.Error("initial tick failed", zap.Error(err))
	}

	for {
		select {
		case <-ctx.Done():
			e.logger.Info("grading engine stopping")
			return ctx.Err()
		case <-ticker.C:
			if _, err := e.tick(ctx, triggerKeys); err != nil {
				e.logger.Error("tick failed", zap.Error(err))
			}
		}
	}
}

// RunOnce processes scan batches back to back until the scanner finds no new
// trigger events, then returns. Used by the replay tests and by `--once`
// (grade everything that is pending and exit, e.g. right after a reset).
func (e *Engine) RunOnce(ctx context.Context) error {
	triggerKeys := e.registry.AllTriggerKeys()
	for {
		found, err := e.tick(ctx, triggerKeys)
		if err != nil {
			return err
		}
		if found == 0 {
			return nil
		}
	}
}

// tick performs one scan-evaluate cycle. It returns the number of trigger
// events the scan found (zero means the grader is caught up) and an error
// when the scan failed or a batch stopped early at a failing event.
func (e *Engine) tick(ctx context.Context, triggerKeys []string) (int, error) {
	// Scan for new triggers
	events, _, err := e.scanner.Scan(ctx, triggerKeys, e.registry.AllTriggerMatchers())
	if err != nil {
		return 0, err
	}

	if len(events) == 0 {
		return 0, nil
	}

	e.logger.Info("processing triggers",
		zap.Int("count", len(events)),
	)

	// Evaluate each trigger, stopping at first failure so the cursor
	// doesn't advance past events that weren't successfully processed.
	var lastSuccessID primitive.ObjectID
	var batchErr error
	anySuccess := false
	for _, event := range events {
		if err := e.evaluator.EvaluateAndStore(ctx, event); err != nil {
			e.logger.Error("evaluation failed, stopping batch",
				zap.String("eventKey", event.EventKey),
				zap.String("user_id", event.UserID),
				zap.Error(err),
			)
			batchErr = err
			break
		}
		lastSuccessID = event.ID
		anySuccess = true
	}

	if !anySuccess {
		// First event failed — don't advance cursor at all
		return len(events), batchErr
	}

	// Update cursor to the last successfully processed event
	if err := e.scanner.UpdateCursor(ctx, lastSuccessID); err != nil {
		e.logger.Error("failed to update cursor", zap.Error(err))
		return len(events), err
	}

	return len(events), batchErr
}
