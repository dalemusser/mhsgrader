// internal/app/bootstrap/config.go
package bootstrap

import (
	"fmt"
	"time"

	"github.com/dalemusser/waffle/config"
	wafflemongo "github.com/dalemusser/waffle/pantry/mongo"
	"go.uber.org/zap"
)

// EnvVarPrefix is the prefix for environment variables.
const EnvVarPrefix = "MHSGRADER"

// appConfigKeys defines the configuration keys for this application.
var appConfigKeys = []config.AppKey{
	{Name: "mongo_uri", Default: "mongodb://localhost:27017", Desc: "MongoDB connection URI"},
	{Name: "mongo_max_pool_size", Default: 100, Desc: "MongoDB max connection pool size (default: 100)"},
	{Name: "mongo_min_pool_size", Default: 10, Desc: "MongoDB min connection pool size (default: 10)"},

	// Database names (same cluster, different databases)
	{Name: "log_database", Default: "stratalog", Desc: "Database for reading logs"},
	{Name: "grades_database", Default: "mhsgrader", Desc: "Database for storing grades"},

	// Grader settings
	{Name: "game", Default: "mhs", Desc: "Game identifier"},
	{Name: "scan_interval", Default: "5s", Desc: "Poll interval for new logs (e.g., 5s, 10s, 1m)"},
	{Name: "batch_size", Default: 500, Desc: "Maximum logs to process per scan"},
}

// LoadConfig loads WAFFLE core config and app-specific config.
func LoadConfig(logger *zap.Logger) (*config.CoreConfig, AppConfig, error) {
	coreCfg, appValues, err := config.LoadWithAppConfig(logger, EnvVarPrefix, appConfigKeys)
	if err != nil {
		return nil, AppConfig{}, err
	}

	appCfg := AppConfig{
		MongoURI:         appValues.String("mongo_uri"),
		MongoMaxPoolSize: uint64(appValues.Int("mongo_max_pool_size")),
		MongoMinPoolSize: uint64(appValues.Int("mongo_min_pool_size")),

		LogDatabase:    appValues.String("log_database"),
		GradesDatabase: appValues.String("grades_database"),

		Game:         appValues.String("game"),
		ScanInterval: appValues.Duration("scan_interval", 5*time.Second),
		BatchSize:    appValues.Int("batch_size"),
	}

	return coreCfg, appCfg, nil
}

// ValidateConfig performs app-specific config validation.
func ValidateConfig(coreCfg *config.CoreConfig, appCfg AppConfig, logger *zap.Logger) error {
	if err := wafflemongo.ValidateURI(appCfg.MongoURI); err != nil {
		logger.Error("invalid MongoDB URI", zap.Error(err))
		return fmt.Errorf("invalid MongoDB URI: %w", err)
	}

	if appCfg.Game == "" {
		return fmt.Errorf("game identifier is required")
	}

	if appCfg.ScanInterval < time.Second {
		return fmt.Errorf("scan_interval must be at least 1 second")
	}

	if appCfg.BatchSize < 1 {
		return fmt.Errorf("batch_size must be at least 1")
	}

	return nil
}
