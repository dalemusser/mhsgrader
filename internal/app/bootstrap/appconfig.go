// internal/app/bootstrap/appconfig.go
package bootstrap

import "time"

// AppConfig holds service-specific configuration for the MHS Grader.
type AppConfig struct {
	// MongoDB connection configuration
	MongoURI         string // MongoDB connection string (e.g., mongodb://localhost:27017)
	MongoMaxPoolSize uint64 // Maximum connections in pool (default: 100)
	MongoMinPoolSize uint64 // Minimum connections to keep warm (default: 10)

	// Database names (same cluster, different databases for separation of concerns)
	LogDatabase    string // Database for reading logs (default: stratalog)
	GradesDatabase string // Database for storing grades (default: mhsgrader)

	// Grader settings
	Game         string        // Game identifier (default: mhs)
	ScanInterval time.Duration // Poll interval for new logs (default: 5s)
	BatchSize    int           // Max logs per scan (default: 500)
	ReprocessAll bool          // Set true to reset cursor and reprocess all logs
}
