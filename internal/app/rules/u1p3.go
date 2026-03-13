package rules

import (
	"context"

	"go.mongodb.org/mongo-driver/mongo"
)

// U1P3Rule — Defend the Expedition: Check for wrong argument selections.
type U1P3Rule struct{ BaseRule }

func NewU1P3Rule() *U1P3Rule {
	return &U1P3Rule{NewBaseRule(1, 3, "v2",
		[]string{"DialogueNodeEvent:30:98"},
		[]string{"questActiveEvent:34"},
	)}
}

func (r *U1P3Rule) Evaluate(ctx context.Context, db *mongo.Database, game, playerID string) (Result, error) {
	helper := NewLogDataHelper(db, game)

	window, err := helper.GetAttemptWindow(ctx, playerID, "questActiveEvent:34")
	if err != nil {
		return Result{}, err
	}
	if window == nil {
		return Flagged("NO_TRIGGER", nil), nil
	}

	yellowNodes := []string{
		"DialogueNodeEvent:70:25",
		"DialogueNodeEvent:70:33",
	}

	hasYellow, err := helper.HasAnyEventInWindow(ctx, playerID, yellowNodes, window)
	if err != nil {
		return Result{}, err
	}

	if hasYellow {
		return Flagged("WRONG_ARG_SELECTED", nil), nil
	}
	return Passed(), nil
}
