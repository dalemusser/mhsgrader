package rules

import (
	"context"

	"go.mongodb.org/mongo-driver/mongo"
)

// U5P1Rule — If I Had a Nickel (Floors 1-2): Success + negative count <= 2.
type U5P1Rule struct{ BaseRule }

func NewU5P1Rule() *U5P1Rule {
	return &U5P1Rule{NewBaseRule(5, 1, "v2",
		[]string{"questActiveEvent:43"},
		[]string{"questFinishEvent:43"},
	)}
}

func (r *U5P1Rule) Evaluate(ctx context.Context, db *mongo.Database, game, playerID string) (Result, error) {
	helper := NewLogDataHelper(db, game)

	window, err := helper.GetAttemptWindow(ctx, playerID, "questFinishEvent:43")
	if err != nil {
		return Result{}, err
	}
	if window == nil {
		return Flagged("NO_TRIGGER", nil), nil
	}

	successKey := "DialogueNodeEvent:100:44"
	hasSuccess, err := helper.HasEventInWindow(ctx, playerID, successKey, window)
	if err != nil {
		return Result{}, err
	}

	negKeys := []string{
		"DialogueNodeEvent:100:38",
		"DialogueNodeEvent:100:39",
		"DialogueNodeEvent:100:43",
	}

	negCount, err := helper.CountEventsInWindow(ctx, playerID, negKeys, window)
	if err != nil {
		return Result{}, err
	}

	if hasSuccess && negCount <= 2 {
		return Passed(), nil
	}
	if !hasSuccess {
		return Flagged("MISSING_SUCCESS_NODE", nil), nil
	}
	return Flagged("TOO_MANY_NEGATIVES", map[string]any{"negativeCount": negCount}), nil
}
