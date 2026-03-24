package rules

import (
	"context"

	"go.mongodb.org/mongo-driver/mongo"
)

// U2P1Rule — Escape the Ruin: Success node + no yellow nodes.
type U2P1Rule struct{ BaseRule }

func NewU2P1Rule() *U2P1Rule {
	return &U2P1Rule{NewBaseRule(2, 1, "v2",
		[]string{"DialogueNodeEvent:18:1"},
		[]string{"questFinishEvent:21"},
	)}
}

func (r *U2P1Rule) Evaluate(ctx context.Context, db *mongo.Database, game, playerID string, ec EvalContext) (Result, error) {
	helper := NewLogDataHelper(db, game)
	window := ec.Window

	successKey := "DialogueNodeEvent:68:29"
	yellowNodes := []string{
		"DialogueNodeEvent:68:22",
		"DialogueNodeEvent:68:23",
		"DialogueNodeEvent:68:27",
		"DialogueNodeEvent:68:28",
		"DialogueNodeEvent:68:31",
	}

	hasSuccess, err := helper.HasEventInWindow(ctx, playerID, successKey, window)
	if err != nil {
		return Result{}, err
	}

	yellowCount, err := helper.CountEventsInWindow(ctx, playerID, yellowNodes, window)
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
