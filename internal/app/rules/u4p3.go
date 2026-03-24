package rules

import (
	"context"

	"go.mongodb.org/mongo-driver/mongo"
)

// U4P3Rule — Power Play (Floors 3-4): soilMachine attempt scoring per floor.
// +1 if floor3 has exactly 1 attempt, +2 if floor4 has 1 attempt, +1 if floor4 has 2.
// Green if score > 1; Flagged if <= 1.
type U4P3Rule struct{ BaseRule }

func NewU4P3Rule() *U4P3Rule {
	return &U4P3Rule{NewBaseRule(4, 3, "v2",
		[]string{"questActiveEvent:48"},
		[]string{"questActiveEvent:50"},
	)}
}

func (r *U4P3Rule) Evaluate(ctx context.Context, db *mongo.Database, game, playerID string, ec EvalContext) (Result, error) {
	helper := NewLogDataHelper(db, game)
	window := ec.Window

	// Count soilMachine events for floor 3, machine 1
	cFloor3, err := helper.CountByEventTypeAndData(ctx, playerID, "soilMachine",
		map[string]string{"machine": "1", "floor": "3"}, window)
	if err != nil {
		return Result{}, err
	}

	// Count soilMachine events for floor 4, machine 1
	cFloor4, err := helper.CountByEventTypeAndData(ctx, playerID, "soilMachine",
		map[string]string{"machine": "1", "floor": "4"}, window)
	if err != nil {
		return Result{}, err
	}

	score := 0
	if cFloor3 == 1 {
		score += 1
	}
	if cFloor4 == 1 {
		score += 2
	} else if cFloor4 == 2 {
		score += 1
	}

	totalAttempts := cFloor3 + cFloor4
	metrics := map[string]any{
		"floor3Attempts": cFloor3,
		"floor4Attempts": cFloor4,
		"score":          score,
		"mistakeCount":   totalAttempts,
	}
	if score > 1 {
		return PassedWithMetrics(metrics), nil
	}
	return Flagged("SCORE_BELOW_THRESHOLD", metrics), nil
}
