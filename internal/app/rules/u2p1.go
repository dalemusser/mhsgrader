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

func (r *U2P1Rule) Evaluate(ctx context.Context, db *mongo.Database, game, playerID string) (Result, error) {
	helper := NewLogDataHelper(db, game)

	window, err := helper.GetAttemptWindow(ctx, playerID, "questFinishEvent:21")
	if err != nil {
		return Result{}, err
	}
	if window == nil {
		return Flagged("NO_TRIGGER", nil), nil
	}

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

	hasYellow, err := helper.HasAnyEventInWindow(ctx, playerID, yellowNodes, window)
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
