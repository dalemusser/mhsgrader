package rules

import (
	"context"

	"go.mongodb.org/mongo-driver/mongo"
)

// U4P1Rule — Well What Have We Here?: Score-based with time component.
// +0.5 if correct choice (88:5), +duration bonus (1.0 if <=30s, 0.5 if 30-90s).
// Green if score >= 1; Flagged if < 1.
type U4P1Rule struct{ BaseRule }

func NewU4P1Rule() *U4P1Rule {
	return &U4P1Rule{NewBaseRule(4, 1, "v2",
		[]string{"DialogueNodeEvent:88:0"},
		[]string{"questActiveEvent:39"},
	)}
}

func (r *U4P1Rule) Evaluate(ctx context.Context, db *mongo.Database, game, playerID string) (Result, error) {
	helper := NewLogDataHelper(db, game)

	window, err := helper.GetAttemptWindow(ctx, playerID, "questActiveEvent:39")
	if err != nil {
		return Result{}, err
	}
	if window == nil {
		return Flagged("NO_TRIGGER", nil), nil
	}

	score := 0.0

	// Check for correct choice
	hasCorrect, err := helper.HasEventInWindow(ctx, playerID, "DialogueNodeEvent:88:5", window)
	if err != nil {
		return Result{}, err
	}
	if hasCorrect {
		score += 0.5
	}

	// Check Soil Key Puzzle timing
	startData := map[string]string{"Soil Key Puzzle Status": "Started"}
	endData := map[string]string{"Soil Key Puzzle Status": "Finished"}

	startEvent, endEvent, err := helper.FindEventPairByEventTypeAndData(ctx, playerID,
		"Soil Key Puzzle", startData, endData, window)
	if err != nil {
		return Result{}, err
	}

	if startEvent != nil && endEvent != nil {
		duration := endEvent.ServerTimestamp.Sub(startEvent.ServerTimestamp).Seconds()
		if duration > 0 && duration <= 30 {
			score += 1.0
		} else if duration > 30 && duration <= 90 {
			score += 0.5
		}
	}

	if score >= 1.0 {
		return Passed(), nil
	}
	return Flagged("SCORE_BELOW_THRESHOLD", map[string]any{"score": score}), nil
}
