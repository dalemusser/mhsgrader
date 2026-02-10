// cmd/mhsgrader/main.go
package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/dalemusser/mhsgrader/internal/app/bootstrap"
	"github.com/dalemusser/mhsgrader/internal/app/grader"
	"github.com/dalemusser/waffle/logging"
	"go.uber.org/zap"
)

func main() {
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

	// Create and start the grading engine
	engine := grader.NewEngine(
		deps.LogDatabase,
		deps.GradesDatabase,
		logger,
		appCfg.Game,
		appCfg.ScanInterval,
		appCfg.BatchSize,
		appCfg.ReprocessAll,
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
