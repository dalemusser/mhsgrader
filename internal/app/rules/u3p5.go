package rules

import (
	"context"

	"go.mongodb.org/mongo-driver/mongo"
)

// U3P5Rule — Part of a Balanced Ecosystem: Weighted pos/neg scoring.
// score = posCount * 1.0 - negCount * 0.5; Green if >= 3.
type U3P5Rule struct{ BaseRule }

func NewU3P5Rule() *U3P5Rule {
	return &U3P5Rule{NewBaseRule(3, 5, "v2",
		[]string{"DialogueNodeEvent:73:200"},
		[]string{"DialogueNodeEvent:10:194"},
	)}
}

func (r *U3P5Rule) Evaluate(ctx context.Context, db *mongo.Database, game, playerID string, ec EvalContext) (Result, error) {
	helper := NewLogDataHelper(db, game)
	window := ec.Window

	posCount, err := helper.CountEventInIDWindow(ctx, playerID, "DialogueNodeEvent:73:163", window)
	if err != nil {
		return Result{}, err
	}

	negKeys := []string{
		"DialogueNodeEvent:73:164",
		"DialogueNodeEvent:73:168",
		"DialogueNodeEvent:73:171",
	}

	negCount, err := helper.CountEventsInWindow(ctx, playerID, negKeys, window)
	if err != nil {
		return Result{}, err
	}

	score := float64(posCount)*1.0 - float64(negCount)*0.5

	metrics := map[string]any{
		"posCount":     posCount,
		"score":        score,
		"mistakeCount": negCount,
	}
	if score >= 3.0 {
		return PassedWithMetrics(metrics), nil
	}
	return Flagged("TOO_MANY_NEGATIVES", metrics), nil
}
