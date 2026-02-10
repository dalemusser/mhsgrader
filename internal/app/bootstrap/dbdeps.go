// internal/app/bootstrap/dbdeps.go
package bootstrap

import (
	"go.mongodb.org/mongo-driver/mongo"
)

// DBDeps holds database and service dependencies for the grader.
type DBDeps struct {
	MongoClient    *mongo.Client
	LogDatabase    *mongo.Database // stratalog - for reading logs (read-only)
	GradesDatabase *mongo.Database // mhsgrader - for storing grades and state
}
