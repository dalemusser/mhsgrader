package rules

import (
	"context"

	"go.mongodb.org/mongo-driver/mongo"
)

// U5P4Rule — Water Problems Require Water Solutions: Success + zero tolerance.
// Green if success present AND zero negative events; Flagged otherwise.
type U5P4Rule struct{ BaseRule }

func NewU5P4Rule() *U5P4Rule {
	return &U5P4Rule{NewBaseRule(5, 4, "v2",
		[]string{"questFinishEvent:44"},
		[]string{"questFinishEvent:45"},
	)}
}

func (r *U5P4Rule) Evaluate(ctx context.Context, db *mongo.Database, game, playerID string, ec EvalContext) (Result, error) {
	helper := NewLogDataHelper(db, game)
	window := ec.Window

	successKey := "DialogueNodeEvent:106:35"
	hasSuccess, err := helper.HasEventInWindow(ctx, playerID, successKey, window)
	if err != nil {
		return Result{}, err
	}

	if !hasSuccess {
		return Flagged("MISSING_SUCCESS_NODE", map[string]any{"mistakeCount": int64(0)}), nil
	}

	negKeys := []string{
		"DialogueNodeEvent:106:4", "DialogueNodeEvent:106:25", "DialogueNodeEvent:106:26",
		"DialogueNodeEvent:106:27", "DialogueNodeEvent:106:28", "DialogueNodeEvent:106:29",
		"DialogueNodeEvent:106:30", "DialogueNodeEvent:106:31", "DialogueNodeEvent:106:32",
		"DialogueNodeEvent:106:33", "DialogueNodeEvent:106:34",
	}

	negCount, err := helper.CountEventsInWindow(ctx, playerID, negKeys, window)
	if err != nil {
		return Result{}, err
	}

	if negCount == 0 {
		return PassedWithMetrics(map[string]any{"mistakeCount": negCount}), nil
	}
	return Flagged("TOO_MANY_NEGATIVES", map[string]any{"mistakeCount": negCount}), nil
}
