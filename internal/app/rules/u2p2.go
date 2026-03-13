package rules

import (
	"context"

	"go.mongodb.org/mongo-driver/mongo"
)

// U2P2Rule — Foraged Forging: Count bad feedback events.
// Green if count <= 1; Flagged if > 1.
type U2P2Rule struct{ BaseRule }

func NewU2P2Rule() *U2P2Rule {
	return &U2P2Rule{NewBaseRule(2, 2, "v2",
		[]string{"questFinishEvent:21"},
		[]string{"DialogueNodeEvent:20:26"},
	)}
}

func (r *U2P2Rule) Evaluate(ctx context.Context, db *mongo.Database, game, playerID string) (Result, error) {
	helper := NewLogDataHelper(db, game)

	window, err := helper.GetAttemptWindow(ctx, playerID, "DialogueNodeEvent:20:26")
	if err != nil {
		return Result{}, err
	}
	if window == nil {
		return Flagged("NO_TRIGGER", nil), nil
	}

	targetKeys := []string{
		"DialogueNodeEvent:18:99",
		"DialogueNodeEvent:28:179",
		"DialogueNodeEvent:59:179",
		"DialogueNodeEvent:18:223",
		"DialogueNodeEvent:28:182",
		"DialogueNodeEvent:59:182",
		"DialogueNodeEvent:18:224",
		"DialogueNodeEvent:28:183",
		"DialogueNodeEvent:59:183",
	}

	count, err := helper.CountEventsInWindow(ctx, playerID, targetKeys, window)
	if err != nil {
		return Result{}, err
	}

	if count <= 1 {
		return Passed(), nil
	}
	return Flagged("BAD_FEEDBACK", map[string]any{"triggering_number": count}), nil
}
