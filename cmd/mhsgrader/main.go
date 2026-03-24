// cmd/mhsgrader/main.go
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/dalemusser/mhsgrader/internal/app/bootstrap"
	"github.com/dalemusser/mhsgrader/internal/app/grader"
	"github.com/dalemusser/mhsgrader/internal/app/store/graderstate"
	"github.com/dalemusser/mhsgrader/internal/app/store/progressgrades"
	"github.com/dalemusser/waffle/logging"
	"github.com/spf13/pflag"
	"go.mongodb.org/mongo-driver/mongo"
	"go.uber.org/zap"
)

func main() {
	// Define reset flag (must be before LoadConfig which calls pflag.Parse())
	pflag.Bool("reset", false, "Reset cursor and clear all grades, then exit")

	// Initialize logger
	logger, err := logging.BuildLogger("info", "dev")
	if err != nil {
		log.Fatalf("failed to initialize logger: %v", err)
	}
	defer logger.Sync()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Load configuration
	coreCfg, appCfg, err := bootstrap.LoadConfig(logger)
	if err != nil {
		logger.Fatal("failed to load config", zap.Error(err))
	}

	// Validate configuration
	if err := bootstrap.ValidateConfig(coreCfg, appCfg, logger); err != nil {
		logger.Fatal("invalid config", zap.Error(err))
	}

	// Connect to database
	deps, err := bootstrap.ConnectDB(ctx, coreCfg, appCfg, logger)
	if err != nil {
		logger.Fatal("failed to connect to database", zap.Error(err))
	}
	defer func() {
		if err := bootstrap.Shutdown(ctx, coreCfg, appCfg, deps, logger); err != nil {
			logger.Error("shutdown error", zap.Error(err))
		}
	}()

	// Ensure schema (indexes)
	if err := bootstrap.EnsureSchema(ctx, coreCfg, appCfg, deps, logger); err != nil {
		logger.Fatal("failed to ensure schema", zap.Error(err))
	}

	// Run startup hook
	if err := bootstrap.Startup(ctx, coreCfg, appCfg, deps, logger); err != nil {
		logger.Fatal("startup failed", zap.Error(err))
	}

	// If reset flag, perform complete cleanup and exit
	resetFlag, _ := pflag.CommandLine.GetBool("reset")
	if resetFlag {
		logger.Info("reset flag set - clearing all grader state and grades")
		if err := performReset(ctx, deps.GradesDatabase, appCfg.Game, logger); err != nil {
			logger.Fatal("failed to reset", zap.Error(err))
		}
		logger.Info("reset complete - exiting")
		return
	}

	// Create and start the grading engine
	engine := grader.NewEngine(
		deps.LogDatabase,
		deps.GradesDatabase,
		logger,
		appCfg.Game,
		appCfg.ScanInterval,
		appCfg.BatchSize,
		appCfg.ActiveGapThreshold,
	)

	// Handle graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigChan
		logger.Info("received shutdown signal")
		cancel()
	}()

	// Run the grading engine (blocks until context is cancelled)
	if err := engine.Run(ctx); err != nil {
		if ctx.Err() == context.Canceled {
			logger.Info("grading engine stopped")
		} else {
			logger.Error("grading engine error", zap.Error(err))
		}
	}

	logger.Info("mhsgrader shutdown complete")
}

// performReset clears all grader state and grades for a fresh start.
func performReset(ctx context.Context, db *mongo.Database, game string, logger *zap.Logger) error {
	graderID := game + "-grader"

	// 1. Delete grader state (cursor)
	stateStore := graderstate.New(db)
	if err := stateStore.Reset(ctx, graderID); err != nil {
		return fmt.Errorf("reset grader state: %w", err)
	}
	logger.Info("cleared grader_state cursor", zap.String("graderID", graderID))

	// 2. Delete grades for this game only
	gradesStore := progressgrades.New(db)
	count, err := gradesStore.DeleteByGame(ctx, game)
	if err != nil {
		return fmt.Errorf("delete grades: %w", err)
	}
	logger.Info("cleared progress_point_grades", zap.String("game", game), zap.Int64("deleted", count))

	return nil
}
