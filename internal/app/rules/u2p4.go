package rules

import (
	"context"

	"go.mongodb.org/mongo-driver/mongo"
)

// U2P4Rule — Investigate the Temple: Success + no bad feedback.
type U2P4Rule struct{ BaseRule }

func NewU2P4Rule() *U2P4Rule {
	return &U2P4Rule{NewBaseRule(2, 4, "v2",
		[]string{"DialogueNodeEvent:22:18"},
		[]string{"DialogueNodeEvent:23:17"},
	)}
}

func (r *U2P4Rule) Evaluate(ctx context.Context, db *mongo.Database, game, playerID string, ec EvalContext) (Result, error) {
	helper := NewLogDataHelper(db, game)
	window := ec.Window

	successKey := "DialogueNodeEvent:74:21"
	badKeys := []string{
		"DialogueNodeEvent:74:16",
		"DialogueNodeEvent:74:17",
		"DialogueNodeEvent:74:20",
		"DialogueNodeEvent:74:22",
	}

	hasSuccess, err := helper.HasEventInWindow(ctx, playerID, successKey, window)
	if err != nil {
		return Result{}, err
	}

	badCount, err := helper.CountEventsInWindow(ctx, playerID, badKeys, window)
	if err != nil {
		return Result{}, err
	}

	if hasSuccess && badCount == 0 {
		return PassedWithMetrics(map[string]any{"mistakeCount": int64(0)}), nil
	}
	if !hasSuccess {
		return Flagged("MISSING_SUCCESS_NODE", map[string]any{"mistakeCount": badCount}), nil
	}
	return Flagged("TOO_MANY_NEGATIVES", map[string]any{"mistakeCount": badCount}), nil
}
