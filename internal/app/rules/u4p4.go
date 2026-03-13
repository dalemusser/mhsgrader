package rules

import (
	"context"

	"go.mongodb.org/mongo-driver/mongo"
)

// U4P4Rule — Power Play (Floor 5) + You Know the Drill: Multi-component scoring.
// soilMachine floor 5 interactions + dialogue success/negative.
// Green if score > 2; Flagged if <= 2.
type U4P4Rule struct{ BaseRule }

func NewU4P4Rule() *U4P4Rule {
	return &U4P4Rule{NewBaseRule(4, 4, "v2",
		[]string{"questActiveEvent:50"},
		[]string{"questActiveEvent:36"},
	)}
}

func (r *U4P4Rule) Evaluate(ctx context.Context, db *mongo.Database, game, playerID string) (Result, error) {
	helper := NewLogDataHelper(db, game)

	window, err := helper.GetAttemptWindow(ctx, playerID, "questActiveEvent:36")
	if err != nil {
		return Result{}, err
	}
	if window == nil {
		return Flagged("NO_TRIGGER", nil), nil
	}

	score := 0

	// Machine 1, floor 5, TopRow
	cM1Top, err := helper.CountByEventTypeAndData(ctx, playerID, "soilMachine",
		map[string]string{"floor": "5", "machine": "1", "row": "TopRow"}, window)
	if err != nil {
		return Result{}, err
	}

	// Machine 1, floor 5, BottomRow
	cM1Bot, err := helper.CountByEventTypeAndData(ctx, playerID, "soilMachine",
		map[string]string{"floor": "5", "machine": "1", "row": "BottomRow"}, window)
	if err != nil {
		return Result{}, err
	}

	if cM1Top == 1 && cM1Bot == 0 {
		score += 1
	}

	// Machine 2, floor 5
	cM2, err := helper.CountByEventTypeAndData(ctx, playerID, "soilMachine",
		map[string]string{"floor": "5", "machine": "2"}, window)
	if err != nil {
		return Result{}, err
	}

	if cM2 == 1 {
		score += 1
	}

	// Dialogue success keys
	successKeys := []string{"DialogueNodeEvent:107:4", "DialogueNodeEvent:107:5"}
	successCount, err := helper.CountEventsInWindow(ctx, playerID, successKeys, window)
	if err != nil {
		return Result{}, err
	}

	// Dialogue negative keys
	negKeys := []string{"DialogueNodeEvent:107:2", "DialogueNodeEvent:107:3", "DialogueNodeEvent:107:6"}
	negCount, err := helper.CountEventsInWindow(ctx, playerID, negKeys, window)
	if err != nil {
		return Result{}, err
	}

	if successCount == 1 && negCount == 0 {
		score += 2
	} else if successCount == 1 && negCount == 1 {
		score += 1
	}

	if score > 2 {
		return Passed(), nil
	}
	return Flagged("SCORE_BELOW_THRESHOLD", map[string]any{"score": score}), nil
}
