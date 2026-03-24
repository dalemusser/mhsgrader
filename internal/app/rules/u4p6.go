package rules

import (
	"context"

	"go.mongodb.org/mongo-driver/mongo"
)

// U4P6Rule — Desert Delicacies: Latest TerasGardenBox placement scoring.
// Score +1 per box with correct soil type. Green if >= 2.
// Box 0 = Gravel, Box 1 = Sand, Box 2 = Clay.
type U4P6Rule struct{ BaseRule }

func NewU4P6Rule() *U4P6Rule {
	return &U4P6Rule{NewBaseRule(4, 6, "v2",
		[]string{"questActiveEvent:41"},
		[]string{"questFinishEvent:56"},
	)}
}

func (r *U4P6Rule) Evaluate(ctx context.Context, db *mongo.Database, game, playerID string, ec EvalContext) (Result, error) {
	helper := NewLogDataHelper(db, game)
	window := ec.Window

	// Expected soil types per box (zero-indexed boxId from game)
	expected := map[string]string{
		"0": "Gravel",
		"1": "Sand",
		"2": "Clay",
	}

	score := 0
	boxDetails := make(map[string]any)
	for boxID, correctSoil := range expected {
		entry, err := helper.FindLatestByEventTypeAndData(ctx, playerID, "TerasGardenBox",
			map[string]string{"actionType": "cameraPlaced", "boxId": boxID}, window)
		if err != nil {
			return Result{}, err
		}
		var placed string
		var correct bool
		if entry != nil {
			if soilType, ok := entry.Data["soilType"].(string); ok {
				placed = soilType
				correct = soilType == correctSoil
				if correct {
					score++
				}
			}
		}
		boxDetails["box"+boxID+"SoilType"] = placed
		boxDetails["box"+boxID+"Correct"] = correct
	}

	mistakeCount := int64(3 - score)
	metrics := map[string]any{
		"score":        score,
		"mistakeCount": mistakeCount,
	}
	for k, v := range boxDetails {
		metrics[k] = v
	}
	if score >= 2 {
		return PassedWithMetrics(metrics), nil
	}
	return Flagged("SCORE_BELOW_THRESHOLD", metrics), nil
}
