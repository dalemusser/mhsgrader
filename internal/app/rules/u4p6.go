package rules

import (
	"context"
	"fmt"

	"go.mongodb.org/mongo-driver/mongo"
)

// U4P6Rule — Desert Delicacies: Latest TerasGardenBox placement scoring.
// Score +1 per box with correct soil type. Green if >= 2.
// Box 1 = Gravel, Box 2 = Sand, Box 3 = Clay.
type U4P6Rule struct{ BaseRule }

func NewU4P6Rule() *U4P6Rule {
	return &U4P6Rule{NewBaseRule(4, 6, "v2",
		[]string{"questActiveEvent:41"},
		[]string{"questFinishEvent:56"},
	)}
}

func (r *U4P6Rule) Evaluate(ctx context.Context, db *mongo.Database, game, playerID string) (Result, error) {
	helper := NewLogDataHelper(db, game)

	window, err := helper.GetAttemptWindow(ctx, playerID, "questFinishEvent:56")
	if err != nil {
		return Result{}, err
	}
	if window == nil {
		return Flagged("NO_TRIGGER", nil), nil
	}

	// Expected soil types per box
	expected := map[string]string{
		"1": "Gravel",
		"2": "Sand",
		"3": "Clay",
	}

	score := 0
	for boxID, correctSoil := range expected {
		entry, err := helper.FindLatestByEventTypeAndData(ctx, playerID, "TerasGardenBox",
			map[string]string{"actionType": "cameraPlaced", "boxId": boxID}, window)
		if err != nil {
			return Result{}, err
		}
		if entry != nil {
			if soilType, ok := entry.Data["soilType"].(string); ok && soilType == correctSoil {
				score++
			}
		}
	}

	if score >= 2 {
		return Passed(), nil
	}
	return Flagged("SCORE_BELOW_THRESHOLD", map[string]any{"score": score, "details": fmt.Sprintf("%d/3 correct", score)}), nil
}
