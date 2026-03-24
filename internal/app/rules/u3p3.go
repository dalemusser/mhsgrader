package rules

import (
	"context"

	"go.mongodb.org/mongo-driver/mongo"
)

// U3P3Rule — Pollution Solution Part II: Score + bonus.
// Base score from incorrect argument count + bonus if backing info used.
// Green if total >= 3; Flagged if < 3.
type U3P3Rule struct{ BaseRule }

func NewU3P3Rule() *U3P3Rule {
	return &U3P3Rule{NewBaseRule(3, 3, "v2",
		[]string{"DialogueNodeEvent:11:34"},
		[]string{"questFinishEvent:18"},
	)}
}

// u3p3BaseScore: 3 if count <= 3, 2 if == 4, 1 if == 5, 0 if >= 6.
func u3p3BaseScore(count int64) int {
	if count <= 3 {
		return 3
	}
	if count == 4 {
		return 2
	}
	if count == 5 {
		return 1
	}
	return 0
}

func (r *U3P3Rule) Evaluate(ctx context.Context, db *mongo.Database, game, playerID string, ec EvalContext) (Result, error) {
	helper := NewLogDataHelper(db, game)
	window := ec.Window

	targetKeys := []string{
		"DialogueNodeEvent:84:20", "DialogueNodeEvent:84:25",
		"DialogueNodeEvent:84:32", "DialogueNodeEvent:84:33", "DialogueNodeEvent:84:34",
		"DialogueNodeEvent:84:35", "DialogueNodeEvent:84:36", "DialogueNodeEvent:84:37",
		"DialogueNodeEvent:84:38", "DialogueNodeEvent:84:39", "DialogueNodeEvent:84:40",
		"DialogueNodeEvent:84:41", "DialogueNodeEvent:84:42", "DialogueNodeEvent:84:43",
		"DialogueNodeEvent:84:44", "DialogueNodeEvent:84:45", "DialogueNodeEvent:84:46",
		"DialogueNodeEvent:84:47",
	}

	count, err := helper.CountEventsInWindow(ctx, playerID, targetKeys, window)
	if err != nil {
		return Result{}, err
	}

	hasBonus, err := helper.HasEventTypeWithDataInWindow(ctx, playerID,
		"argumentationToolEvent", "toolName", "BackingInfoPanel - Pollution Site Data", window)
	if err != nil {
		return Result{}, err
	}

	baseScore := u3p3BaseScore(count)
	totalScore := baseScore
	if hasBonus {
		totalScore++
	}

	metrics := map[string]any{
		"baseScore":       baseScore,
		"usedBackingInfo": hasBonus,
		"totalScore":      totalScore,
		"mistakeCount":    count,
	}
	if totalScore >= 3 {
		return PassedWithMetrics(metrics), nil
	}

	// Distinguish reason: if backing info wasn't checked, flag that specifically
	if !hasBonus {
		return Flagged("MISSING_SUCCESS_NODE", metrics), nil
	}
	return Flagged("WRONG_ARG_SELECTED", metrics), nil
}
