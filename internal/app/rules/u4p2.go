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

func (r *U4P2Rule) Evaluate(ctx context.Context, db *mongo.Database, game, playerID string) (Result, error) {
	helper := NewLogDataHelper(db, game)

	window, err := helper.GetAttemptWindow(ctx, playerID, "questActiveEvent:48")
	if err != nil {
		return Result{}, err
	}
	if window == nil {
		return Flagged("NO_TRIGGER", nil), nil
	}

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

	hasYellow, err := helper.HasAnyEventInWindow(ctx, playerID, yellowKeys, window)
	if err != nil {
		return Result{}, err
	}

	if hasSuccess && !hasYellow {
		return Passed(), nil
	}
	if !hasSuccess {
		return Flagged("MISSING_SUCCESS_NODE", nil), nil
	}
	return Flagged("TOO_MANY_NEGATIVES", nil), nil
}
