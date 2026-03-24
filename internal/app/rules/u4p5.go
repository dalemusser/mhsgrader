package rules

import (
	"context"

	"go.mongodb.org/mongo-driver/mongo"
)

// U4P5Rule — Saving Cadet Anderson: Success + negative count < 4.
type U4P5Rule struct{ BaseRule }

func NewU4P5Rule() *U4P5Rule {
	return &U4P5Rule{NewBaseRule(4, 5, "v2",
		[]string{"questActiveEvent:36"},
		[]string{"questActiveEvent:41"},
	)}
}

func (r *U4P5Rule) Evaluate(ctx context.Context, db *mongo.Database, game, playerID string, ec EvalContext) (Result, error) {
	helper := NewLogDataHelper(db, game)
	window := ec.Window

	posKeys := []string{"DialogueNodeEvent:90:50", "DialogueNodeEvent:90:57"}
	hasPos, err := helper.HasAnyEventInWindow(ctx, playerID, posKeys, window)
	if err != nil {
		return Result{}, err
	}

	if !hasPos {
		return Flagged("MISSING_SUCCESS_NODE", map[string]any{"mistakeCount": int64(0)}), nil
	}

	negKeys := []string{
		"DialogueNodeEvent:90:25", "DialogueNodeEvent:90:37", "DialogueNodeEvent:90:39",
		"DialogueNodeEvent:90:45", "DialogueNodeEvent:90:47", "DialogueNodeEvent:90:52",
		"DialogueNodeEvent:90:54", "DialogueNodeEvent:90:55", "DialogueNodeEvent:90:56",
		"DialogueNodeEvent:90:58", "DialogueNodeEvent:90:59", "DialogueNodeEvent:90:60",
		"DialogueNodeEvent:90:61",
	}

	negCount, err := helper.CountEventsInWindow(ctx, playerID, negKeys, window)
	if err != nil {
		return Result{}, err
	}

	if negCount >= 4 {
		return Flagged("TOO_MANY_NEGATIVES", map[string]any{"mistakeCount": negCount}), nil
	}
	return PassedWithMetrics(map[string]any{"mistakeCount": negCount}), nil
}
