// internal/app/bootstrap/shutdown.go
package bootstrap

import (
	"context"
	"time"

	"github.com/dalemusser/waffle/config"
	"go.uber.org/zap"
)

// Shutdown gracefully shuts down the application.
func Shutdown(ctx context.Context, coreCfg *config.CoreConfig, appCfg AppConfig, deps DBDeps, logger *zap.Logger) error {
	logger.Info("mhsgrader shutting down")

	// Disconnect MongoDB
	if deps.MongoClient != nil {
		disconnectCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := deps.MongoClient.Disconnect(disconnectCtx); err != nil {
			logger.Error("failed to disconnect MongoDB", zap.Error(err))
			return err
		}
		logger.Info("disconnected from MongoDB")
	}

	return nil
}
