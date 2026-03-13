package rules

import (
	"context"

	"go.mongodb.org/mongo-driver/mongo"
)

// U2P7Rule — Which Watershed? Part II: Success + negative count <= 3.
type U2P7Rule struct{ BaseRule }

func NewU2P7Rule() *U2P7Rule {
	return &U2P7Rule{NewBaseRule(2, 7, "v2",
		[]string{"DialogueNodeEvent:20:46"},
		[]string{"questFinishEvent:54"},
	)}
}

func (r *U2P7Rule) Evaluate(ctx context.Context, db *mongo.Database, game, playerID string) (Result, error) {
	helper := NewLogDataHelper(db, game)

	window, err := helper.GetAttemptWindow(ctx, playerID, "questFinishEvent:54")
	if err != nil {
		return Result{}, err
	}
	if window == nil {
		return Flagged("NO_TRIGGER", nil), nil
	}

	successKey := "DialogueNodeEvent:27:7"
	negKeys := []string{
		"DialogueNodeEvent:27:11", "DialogueNodeEvent:27:12", "DialogueNodeEvent:27:13",
		"DialogueNodeEvent:27:14", "DialogueNodeEvent:27:15", "DialogueNodeEvent:27:16",
		"DialogueNodeEvent:27:17", "DialogueNodeEvent:27:18", "DialogueNodeEvent:27:19",
		"DialogueNodeEvent:27:20", "DialogueNodeEvent:27:21", "DialogueNodeEvent:27:22",
		"DialogueNodeEvent:27:23", "DialogueNodeEvent:27:24", "DialogueNodeEvent:27:25",
		"DialogueNodeEvent:27:26", "DialogueNodeEvent:27:27", "DialogueNodeEvent:27:28",
		"DialogueNodeEvent:27:29", "DialogueNodeEvent:27:30",
	}

	hasSuccess, err := helper.HasEventInWindow(ctx, playerID, successKey, window)
	if err != nil {
		return Result{}, err
	}

	negCount, err := helper.CountEventsInWindow(ctx, playerID, negKeys, window)
	if err != nil {
		return Result{}, err
	}

	if hasSuccess && negCount <= 3 {
		return Passed(), nil
	}
	return Flagged("WRONG_ARG_SELECTED", map[string]any{"attempt_number": negCount}), nil
}
