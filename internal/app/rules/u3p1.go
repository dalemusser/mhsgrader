package rules

import (
	"context"

	"go.mongodb.org/mongo-driver/mongo"
)

// U3P1Rule — Supply Run: Count target events.
// Trigger changed from questFinishEvent:17 to DialogueNodeEvent:11:22.
// Green if count > 1; Flagged if <= 1.
type U3P1Rule struct{ BaseRule }

func NewU3P1Rule() *U3P1Rule {
	return &U3P1Rule{NewBaseRule(3, 1, "v2",
		[]string{"DialogueNodeEvent:10:1"},
		[]string{"DialogueNodeEvent:11:22"},
	)}
}

func (r *U3P1Rule) Evaluate(ctx context.Context, db *mongo.Database, game, playerID string, ec EvalContext) (Result, error) {
	helper := NewLogDataHelper(db, game)
	window := ec.Window

	count, err := helper.CountEventInIDWindow(ctx, playerID, "DialogueNodeEvent:10:30", window)
	if err != nil {
		return Result{}, err
	}

	metrics := map[string]any{
		"count":        count,
		"mistakeCount": 3 - count,
	}
	if count > 1 {
		metrics["mistakeCount"] = int64(0)
		return PassedWithMetrics(metrics), nil
	}
	return Flagged("TOO_MANY_NEGATIVES", metrics), nil
}
