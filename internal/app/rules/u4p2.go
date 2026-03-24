package rules

import (
	"context"

	"go.mongodb.org/mongo-driver/mongo"
)

// U4P2Rule — Power Play (Floors 1-2): Success + no yellow feedback.
type U4P2Rule struct{ BaseRule }

func NewU4P2Rule() *U4P2Rule {
	return &U4P2Rule{NewBaseRule(4, 2, "v2",
		[]string{"questActiveEvent:39"},
		[]string{"questActiveEvent:48"},
	)}
}

func (r *U4P2Rule) Evaluate(ctx context.Context, db *mongo.Database, game, playerID string, ec EvalContext) (Result, error) {
	helper := NewLogDataHelper(db, game)
	window := ec.Window

	successKey := "DialogueNodeEvent:88:11"
	yellowKeys := []string{
		"DialogueNodeEvent:102:9",
		"DialogueNodeEvent:102:10",
		"DialogueNodeEvent:102:12",
		"DialogueNodeEvent:102:18",
		"DialogueNodeEvent:102:23",
	}

	hasSuccess, err := helper.HasEventInWindow(ctx, playerID, successKey, window)
	if err != nil {
		return Result{}, err
	}

	yellowCount, err := helper.CountEventsInWindow(ctx, playerID, yellowKeys, window)
	if err != nil {
		return Result{}, err
	}

	if hasSuccess && yellowCount == 0 {
		return PassedWithMetrics(map[string]any{"mistakeCount": int64(0)}), nil
	}
	if !hasSuccess {
		return Flagged("MISSING_SUCCESS_NODE", map[string]any{"mistakeCount": yellowCount}), nil
	}
	return Flagged("TOO_MANY_NEGATIVES", map[string]any{"mistakeCount": yellowCount}), nil
}
