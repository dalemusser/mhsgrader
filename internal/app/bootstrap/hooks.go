// internal/app/bootstrap/hooks.go
package bootstrap

import (
	"net/http"

	"github.com/dalemusser/waffle/app"
	"github.com/dalemusser/waffle/config"
	"go.uber.org/zap"
)

// buildNoOpHandler returns a no-op HTTP handler since mhsgrader doesn't serve HTTP.
func buildNoOpHandler(coreCfg *config.CoreConfig, appCfg AppConfig, deps DBDeps, logger *zap.Logger) (http.Handler, error) {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("mhsgrader is running"))
	}), nil
}

// Hooks wires this app into the WAFFLE lifecycle.
var Hooks = app.Hooks[AppConfig, DBDeps]{
	Name:           "mhsgrader",
	LoadConfig:     LoadConfig,
	ValidateConfig: ValidateConfig,
	ConnectDB:      ConnectDB,
	EnsureSchema:   EnsureSchema,
	Startup:        Startup,
	BuildHandler:   buildNoOpHandler,
	Shutdown:       Shutdown,
}
