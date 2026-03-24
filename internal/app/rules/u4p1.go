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

func (r *U4P1Rule) Evaluate(ctx context.Context, db *mongo.Database, game, playerID string, ec EvalContext) (Result, error) {
	helper := NewLogDataHelper(db, game)
	window := ec.Window

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

	var puzzleDurationSecs float64
	var durationBonus float64
	if startEvent != nil && endEvent != nil {
		puzzleDurationSecs = endEvent.ServerTimestamp.Sub(startEvent.ServerTimestamp).Seconds()
		if puzzleDurationSecs > 0 && puzzleDurationSecs <= 30 {
			durationBonus = 1.0
		} else if puzzleDurationSecs > 30 && puzzleDurationSecs <= 90 {
			durationBonus = 0.5
		}
		score += durationBonus
	}

	mistakeCount := int64(0)
	if !hasCorrect {
		mistakeCount = 1
	}

	metrics := map[string]any{
		"hasCorrectAnswer":   hasCorrect,
		"puzzleDurationSecs": puzzleDurationSecs,
		"durationBonus":      durationBonus,
		"score":              score,
		"mistakeCount":       mistakeCount,
	}
	if score >= 1.0 {
		return PassedWithMetrics(metrics), nil
	}
	return Flagged("SCORE_BELOW_THRESHOLD", metrics), nil
}
