// internal/app/rules/u3p3.go
package rules

import (
	"context"

	"go.mongodb.org/mongo-driver/mongo"
)

// U3P3Rule: "Pollution argument"
// Trigger: questFinishEvent:18
// Logic: Attempt-based scoring with bonus
//   - Count TARGET_KEYS (16 keys: 84:20, 84:25, 84:32-47)
//   - baseScore: 3 if <=3, 2 if ==4, 1 if ==5, 0 if >=6
//   - hasBonus: eventType="argumentationToolEvent" AND data.toolName="BackingInfoPanel - Pollution Site Data"
//   - totalScore = baseScore + (1 if hasBonus else 0)
// Green: totalScore >= 3
// Yellow: totalScore < 3
type U3P3Rule struct {
	BaseRule
}

// NewU3P3Rule creates a new U3P3 rule.
func NewU3P3Rule() *U3P3Rule {
	return &U3P3Rule{
		BaseRule: NewBaseRule(3, 3, "v1", []string{"questFinishEvent:18"}),
	}
}

const u3p3Threshold = 3

// baseScore calculates base score from target count:
// 3 if count <= 3, 2 if count == 4, 1 if count == 5, 0 if count >= 6
func baseScore(count int64) int {
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

// Evaluate calculates score with bonus.
func (r *U3P3Rule) Evaluate(ctx context.Context, db *mongo.Database, game, playerID string) (Result, error) {
	helper := NewLogDataHelper(db, game)

	// Get the attempt window for this trigger
	triggerKey := "questFinishEvent:18"
	window, err := helper.GetAttemptWindow(ctx, playerID, triggerKey)
	if err != nil {
		return Result{}, err
	}
	if window == nil {
		return Yellow("NO_TRIGGER", map[string]any{
			"reason": "No trigger event found",
		}), nil
	}

	// Target keys: 84:20, 84:25, 84:32-47 (16 keys)
	targetKeys := []string{
		"DialogueNodeEvent:84:20",
		"DialogueNodeEvent:84:25",
		"DialogueNodeEvent:84:32",
		"DialogueNodeEvent:84:33",
		"DialogueNodeEvent:84:34",
		"DialogueNodeEvent:84:35",
		"DialogueNodeEvent:84:36",
		"DialogueNodeEvent:84:37",
		"DialogueNodeEvent:84:38",
		"DialogueNodeEvent:84:39",
		"DialogueNodeEvent:84:40",
		"DialogueNodeEvent:84:41",
		"DialogueNodeEvent:84:42",
		"DialogueNodeEvent:84:43",
		"DialogueNodeEvent:84:44",
		"DialogueNodeEvent:84:45",
		"DialogueNodeEvent:84:46",
		"DialogueNodeEvent:84:47",
	}

	count, err := helper.CountEventsInWindow(ctx, playerID, targetKeys, window)
	if err != nil {
		return Result{}, err
	}

	// Check for bonus: eventType="argumentationToolEvent" AND data.toolName="BackingInfoPanel - Pollution Site Data"
	hasBonus, err := helper.HasEventTypeWithDataInWindow(ctx, playerID,
		"argumentationToolEvent",
		"toolName",
		"BackingInfoPanel - Pollution Site Data",
		window)
	if err != nil {
		return Result{}, err
	}

	// Calculate total score
	score := baseScore(count)
	if hasBonus {
		score++
	}

	if score < u3p3Threshold {
		return Yellow("LOW_SCORE", map[string]any{
			"score":       score,
			"targetCount": count,
			"hasBonus":    hasBonus,
			"threshold":   u3p3Threshold,
		}), nil
	}

	return Green(), nil
}
