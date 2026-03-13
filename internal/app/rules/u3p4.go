package rules

import (
	"context"

	"go.mongodb.org/mongo-driver/mongo"
)

// U3P4Rule — Forsaken Facility: Gate + count scoring.
// Must have gate key; score based on target count.
type U3P4Rule struct{ BaseRule }

func NewU3P4Rule() *U3P4Rule {
	return &U3P4Rule{NewBaseRule(3, 4, "v2",
		[]string{"questFinishEvent:18"},
		[]string{"DialogueNodeEvent:73:200"},
	)}
}

func (r *U3P4Rule) Evaluate(ctx context.Context, db *mongo.Database, game, playerID string) (Result, error) {
	helper := NewLogDataHelper(db, game)

	window, err := helper.GetAttemptWindow(ctx, playerID, "DialogueNodeEvent:73:200")
	if err != nil {
		return Result{}, err
	}
	if window == nil {
		return Flagged("NO_TRIGGER", nil), nil
	}

	hasGate, err := helper.HasEventInWindow(ctx, playerID, "DialogueNodeEvent:78:24", window)
	if err != nil {
		return Result{}, err
	}
	if !hasGate {
		return Flagged("MISSING_SUCCESS_NODE", nil), nil
	}

	targetKeys := []string{
		"DialogueNodeEvent:78:3", "DialogueNodeEvent:78:4", "DialogueNodeEvent:78:7",
		"DialogueNodeEvent:78:9", "DialogueNodeEvent:78:10", "DialogueNodeEvent:78:12",
		"DialogueNodeEvent:78:18", "DialogueNodeEvent:78:23",
	}

	count, err := helper.CountEventsInWindow(ctx, playerID, targetKeys, window)
	if err != nil {
		return Result{}, err
	}

	// score: 2 if count == 0, 1 if count <= 2, 0 if count >= 3
	if count <= 2 {
		return Passed(), nil
	}
	return Flagged("TOO_MANY_NEGATIVES", map[string]any{"attempt_number": count}), nil
}
