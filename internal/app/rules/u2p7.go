// internal/app/rules/u2p7.go
package rules

import (
	"context"

	"go.mongodb.org/mongo-driver/mongo"
)

// U2P7Rule: "Watershed Argument"
// Trigger: questFinishEvent:54
// Logic: Attempt-based
//   - Must have success DialogueNodeEvent:27:7 in window
//   - Count NEG_KEYS (20 keys: 27:11 through 27:30)
// Green: hasSuccess && negCount <= 3
// Yellow: otherwise
type U2P7Rule struct {
	BaseRule
}

// NewU2P7Rule creates a new U2P7 rule.
func NewU2P7Rule() *U2P7Rule {
	return &U2P7Rule{
		BaseRule: NewBaseRule(2, 7, "v1", []string{"questFinishEvent:54"}),
	}
}

const u2p7NegativeThreshold = 3

// Evaluate checks success and negative event count.
func (r *U2P7Rule) Evaluate(ctx context.Context, db *mongo.Database, game, playerID string) (Result, error) {
	helper := NewLogDataHelper(db, game)

	// Get the attempt window for this trigger
	triggerKey := "questFinishEvent:54"
	window, err := helper.GetAttemptWindow(ctx, playerID, triggerKey)
	if err != nil {
		return Result{}, err
	}
	if window == nil {
		return Yellow("NO_TRIGGER", map[string]any{
			"reason": "No trigger event found",
		}), nil
	}

	// Check for success key in window
	successKey := "DialogueNodeEvent:27:7"
	hasSuccess, err := helper.HasEventInWindow(ctx, playerID, successKey, window)
	if err != nil {
		return Result{}, err
	}

	// Negative keys: 27:11 through 27:30 (20 keys)
	negKeys := []string{
		"DialogueNodeEvent:27:11",
		"DialogueNodeEvent:27:12",
		"DialogueNodeEvent:27:13",
		"DialogueNodeEvent:27:14",
		"DialogueNodeEvent:27:15",
		"DialogueNodeEvent:27:16",
		"DialogueNodeEvent:27:17",
		"DialogueNodeEvent:27:18",
		"DialogueNodeEvent:27:19",
		"DialogueNodeEvent:27:20",
		"DialogueNodeEvent:27:21",
		"DialogueNodeEvent:27:22",
		"DialogueNodeEvent:27:23",
		"DialogueNodeEvent:27:24",
		"DialogueNodeEvent:27:25",
		"DialogueNodeEvent:27:26",
		"DialogueNodeEvent:27:27",
		"DialogueNodeEvent:27:28",
		"DialogueNodeEvent:27:29",
		"DialogueNodeEvent:27:30",
	}

	negCount, err := helper.CountEventsInWindow(ctx, playerID, negKeys, window)
	if err != nil {
		return Result{}, err
	}

	// Green if hasSuccess and negCount <= 3
	if hasSuccess && negCount <= u2p7NegativeThreshold {
		return Green(), nil
	}

	return Yellow("TOO_MANY_NEGATIVES", map[string]any{
		"hasSuccess":    hasSuccess,
		"negativeCount": negCount,
		"threshold":     u2p7NegativeThreshold,
	}), nil
}
