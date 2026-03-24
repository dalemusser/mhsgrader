package rules

import (
	"context"

	"go.mongodb.org/mongo-driver/mongo"
)

// U2P3Rule — Getting the Band Back Together Part II: Count wrong-direction prompts.
// Uses start/end windowing with DialogueNodeEvent:20:33 as activity start.
// Green if count <= 6; Flagged if > 6.
type U2P3Rule struct{ BaseRule }

func NewU2P3Rule() *U2P3Rule {
	return &U2P3Rule{NewBaseRule(2, 3, "v2",
		[]string{"DialogueNodeEvent:20:33"},
		[]string{"DialogueNodeEvent:22:18"},
	)}
}

func (r *U2P3Rule) Evaluate(ctx context.Context, db *mongo.Database, game, playerID string, ec EvalContext) (Result, error) {
	helper := NewLogDataHelper(db, game)
	window := ec.Window

	targetKeys := []string{
		"DialogueNodeEvent:18:225", "DialogueNodeEvent:28:185", "DialogueNodeEvent:59:185",
		"DialogueNodeEvent:28:184", "DialogueNodeEvent:28:191", "DialogueNodeEvent:59:184", "DialogueNodeEvent:59:191",
		"DialogueNodeEvent:18:226", "DialogueNodeEvent:18:227", "DialogueNodeEvent:28:186", "DialogueNodeEvent:59:186",
		"DialogueNodeEvent:18:228", "DialogueNodeEvent:28:187", "DialogueNodeEvent:59:187",
		"DialogueNodeEvent:18:229", "DialogueNodeEvent:28:188", "DialogueNodeEvent:59:188",
		"DialogueNodeEvent:18:230", "DialogueNodeEvent:28:180", "DialogueNodeEvent:59:180",
		"DialogueNodeEvent:18:233", "DialogueNodeEvent:28:192", "DialogueNodeEvent:59:192",
		"DialogueNodeEvent:18:234", "DialogueNodeEvent:28:193", "DialogueNodeEvent:59:193",
		"DialogueNodeEvent:18:235", "DialogueNodeEvent:28:194", "DialogueNodeEvent:59:194",
		"DialogueNodeEvent:18:236", "DialogueNodeEvent:18:237", "DialogueNodeEvent:28:190", "DialogueNodeEvent:59:190",
	}

	count, err := helper.CountEventsInWindow(ctx, playerID, targetKeys, window)
	if err != nil {
		return Result{}, err
	}

	if count <= 6 {
		return PassedWithMetrics(map[string]any{"mistakeCount": count}), nil
	}
	return Flagged("BAD_FEEDBACK", map[string]any{"mistakeCount": count}), nil
}
