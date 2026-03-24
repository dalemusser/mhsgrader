// internal/app/bootstrap/db.go
package bootstrap

import (
	"context"

	"github.com/dalemusser/waffle/config"
	wafflemongo "github.com/dalemusser/waffle/pantry/mongo"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.uber.org/zap"
)

// ConnectDB connects to MongoDB and sets up both databases.
func ConnectDB(ctx context.Context, coreCfg *config.CoreConfig, appCfg AppConfig, logger *zap.Logger) (DBDeps, error) {
	// Configure MongoDB connection pool
	poolCfg := wafflemongo.DefaultPoolConfig()
	if appCfg.MongoMaxPoolSize > 0 {
		poolCfg.MaxPoolSize = appCfg.MongoMaxPoolSize
	}
	if appCfg.MongoMinPoolSize > 0 {
		poolCfg.MinPoolSize = appCfg.MongoMinPoolSize
	}

	// Connect to the cluster (using log database as the default for auth)
	client, err := wafflemongo.ConnectWithPool(ctx, appCfg.MongoURI, appCfg.LogDatabase, poolCfg)
	if err != nil {
		return DBDeps{}, err
	}

	// Get references to both databases on the same cluster
	logDB := client.Database(appCfg.LogDatabase)
	gradesDB := client.Database(appCfg.GradesDatabase)

	logger.Info("connected to MongoDB",
		zap.String("log_database", appCfg.LogDatabase),
		zap.String("grades_database", appCfg.GradesDatabase),
		zap.Uint64("max_pool_size", poolCfg.MaxPoolSize),
		zap.Uint64("min_pool_size", poolCfg.MinPoolSize),
	)

	return DBDeps{
		MongoClient:    client,
		LogDatabase:    logDB,
		GradesDatabase: gradesDB,
	}, nil
}

// EnsureSchema sets up indexes for the grader collections.
func EnsureSchema(ctx context.Context, coreCfg *config.CoreConfig, appCfg AppConfig, deps DBDeps, logger *zap.Logger) error {
	logger.Info("ensuring database indexes",
		zap.String("grades_database", appCfg.GradesDatabase),
		zap.String("log_database", appCfg.LogDatabase),
	)

	// Ensure progress_point_grades index (in grades database)
	if err := ensureProgressGradesIndexes(ctx, deps.GradesDatabase); err != nil {
		logger.Error("failed to ensure progress_point_grades indexes", zap.Error(err))
		return err
	}

	// Ensure grader_state index (in grades database)
	if err := ensureGraderStateIndexes(ctx, deps.GradesDatabase); err != nil {
		logger.Error("failed to ensure grader_state indexes", zap.Error(err))
		return err
	}

	// Ensure logdata indexes for grading queries (in log database)
	if err := ensureLogdataIndexes(ctx, deps.LogDatabase); err != nil {
		logger.Error("failed to ensure logdata indexes", zap.Error(err))
		return err
	}

	logger.Info("database schema ensured successfully")
	return nil
}

func ensureProgressGradesIndexes(ctx context.Context, db *mongo.Database) error {
	coll := db.Collection("progress_point_grades")
	_, err := coll.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys: bson.D{
			{Key: "game", Value: 1},
			{Key: "playerId", Value: 1},
		},
		Options: options.Index().SetUnique(true).SetName("uniq_progress_game_player"),
	})
	return err
}

func ensureGraderStateIndexes(ctx context.Context, db *mongo.Database) error {
	// grader_state uses _id as the key, no additional indexes needed
	return nil
}

// ensureLogdataIndexes creates indexes on logdata collection for efficient grading queries.
// These indexes are on the stratalog database but are needed for mhsgrader's queries.
func ensureLogdataIndexes(ctx context.Context, db *mongo.Database) error {
	coll := db.Collection("logdata")

	indexes := []mongo.IndexModel{
		// Trigger scanning: game + eventKey + _id for cursor-based scanning
		{
			Keys: bson.D{
				{Key: "game", Value: 1},
				{Key: "eventKey", Value: 1},
				{Key: "_id", Value: 1},
			},
			Options: options.Index().SetName("idx_logdata_grader_trigger_scan"),
		},
		// Windowed grading queries: player + eventKey within time window
		{
			Keys: bson.D{
				{Key: "game", Value: 1},
				{Key: "playerId", Value: 1},
				{Key: "eventKey", Value: 1},
				{Key: "serverTimestamp", Value: 1},
			},
			Options: options.Index().SetName("idx_logdata_grader_player_event_ts"),
		},
		// _id-windowed grading queries: player + eventKey with _id range
		// Used by GetLatestByEventKeysBefore, ExistsByEventKeyInIDWindow,
		// CountByEventKeyInIDWindow, CountByEventKeysInIDWindow, etc.
		{
			Keys: bson.D{
				{Key: "game", Value: 1},
				{Key: "playerId", Value: 1},
				{Key: "eventKey", Value: 1},
				{Key: "_id", Value: 1},
			},
			Options: options.Index().SetName("idx_logdata_grader_player_event_id"),
		},
		// _id-windowed player queries without eventKey filter
		// Used by FindAllInIDWindow for active duration calculation
		{
			Keys: bson.D{
				{Key: "game", Value: 1},
				{Key: "playerId", Value: 1},
				{Key: "_id", Value: 1},
			},
			Options: options.Index().SetName("idx_logdata_grader_player_id"),
		},
		// _id-windowed eventType queries for machine/box interaction grading
		// Used by CountByEventTypeAndDataInIDWindow, FindLatestByEventTypeAndDataInIDWindow, etc.
		{
			Keys: bson.D{
				{Key: "game", Value: 1},
				{Key: "playerId", Value: 1},
				{Key: "eventType", Value: 1},
				{Key: "_id", Value: 1},
			},
			Options: options.Index().SetName("idx_logdata_grader_player_type_id"),
		},
	}

	for _, idx := range indexes {
		_, err := coll.Indexes().CreateOne(ctx, idx)
		if err != nil {
			return err
		}
	}

	return nil
}
