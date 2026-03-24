package rules

import (
	"context"

	"go.mongodb.org/mongo-driver/mongo"
)

// U2P6Rule — Which Watershed? Part I: Pass node + no yellow nodes.
type U2P6Rule struct{ BaseRule }

func NewU2P6Rule() *U2P6Rule {
	return &U2P6Rule{NewBaseRule(2, 6, "v2",
		[]string{"DialogueNodeEvent:23:42"},
		[]string{"DialogueNodeEvent:20:46"},
	)}
}

func (r *U2P6Rule) Evaluate(ctx context.Context, db *mongo.Database, game, playerID string, ec EvalContext) (Result, error) {
	helper := NewLogDataHelper(db, game)
	window := ec.Window

	passKey := "DialogueNodeEvent:20:43"
	yellowNodes := []string{
		"DialogueNodeEvent:20:44",
		"DialogueNodeEvent:20:45",
	}

	hasPass, err := helper.HasEventInWindow(ctx, playerID, passKey, window)
	if err != nil {
		return Result{}, err
	}

	yellowCount, err := helper.CountEventsInWindow(ctx, playerID, yellowNodes, window)
	if err != nil {
		return Result{}, err
	}

	if hasPass && yellowCount == 0 {
		return PassedWithMetrics(map[string]any{"mistakeCount": int64(0)}), nil
	}
	if !hasPass {
		return Flagged("MISSING_SUCCESS_NODE", map[string]any{"mistakeCount": yellowCount}), nil
	}
	return Flagged("HIT_YELLOW_NODE", map[string]any{"mistakeCount": yellowCount}), nil
}
