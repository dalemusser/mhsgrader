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

func (r *U2P4Rule) Evaluate(ctx context.Context, db *mongo.Database, game, playerID string) (Result, error) {
	helper := NewLogDataHelper(db, game)

	window, err := helper.GetAttemptWindow(ctx, playerID, "DialogueNodeEvent:23:17")
	if err != nil {
		return Result{}, err
	}
	if window == nil {
		return Flagged("NO_TRIGGER", nil), nil
	}

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

	hasBad, err := helper.HasAnyEventInWindow(ctx, playerID, badKeys, window)
	if err != nil {
		return Result{}, err
	}

	if hasSuccess && !hasBad {
		return Passed(), nil
	}
	if !hasSuccess {
		return Flagged("MISSING_SUCCESS_NODE", nil), nil
	}
	return Flagged("TOO_MANY_NEGATIVES", nil), nil
}
