package rules

import (
	"context"

	"go.mongodb.org/mongo-driver/mongo"
)

// U2P6Rule — Which Watershed? Part I: Pass node + no yellow nodes.
type U2P6Rule struct{ BaseRule }

func NewU2P6Rule() *U2P6Rule {
	return &U2P6Rule{NewBaseRule(2, 6, "v2",
		[]string{"DialogueNodeEvent:23:42"},
		[]string{"DialogueNodeEvent:20:46"},
	)}
}

func (r *U2P6Rule) Evaluate(ctx context.Context, db *mongo.Database, game, playerID string) (Result, error) {
	helper := NewLogDataHelper(db, game)

	window, err := helper.GetAttemptWindow(ctx, playerID, "DialogueNodeEvent:20:46")
	if err != nil {
		return Result{}, err
	}
	if window == nil {
		return Flagged("NO_TRIGGER", nil), nil
	}

	passKey := "DialogueNodeEvent:20:43"
	yellowNodes := []string{
		"DialogueNodeEvent:20:44",
		"DialogueNodeEvent:20:45",
	}

	hasPass, err := helper.HasEventInWindow(ctx, playerID, passKey, window)
	if err != nil {
		return Result{}, err
	}

	hasYellow, err := helper.HasAnyEventInWindow(ctx, playerID, yellowNodes, window)
	if err != nil {
		return Result{}, err
	}

	if hasPass && !hasYellow {
		return Passed(), nil
	}
	return Flagged("HIT_YELLOW_NODE", nil), nil
}
