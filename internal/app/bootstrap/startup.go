// internal/app/bootstrap/startup.go
package bootstrap

import (
	"context"

	"github.com/dalemusser/waffle/config"
	"go.uber.org/zap"
)

// Startup performs application initialization after database connections are established.
// For the grader, this is where we start the grading engine.
func Startup(ctx context.Context, coreCfg *config.CoreConfig, appCfg AppConfig, deps DBDeps, logger *zap.Logger) error {
	logger.Info("mhsgrader starting",
		zap.String("game", appCfg.Game),
		zap.Duration("scan_interval", appCfg.ScanInterval),
		zap.Int("batch_size", appCfg.BatchSize),
		zap.Bool("reprocess_all", appCfg.ReprocessAll),
	)

	// The grading engine will be started after hooks complete
	// This is handled in main.go since we need to run the engine in the foreground
	return nil
}
