// internal/app/rules/u3p1.go
package rules

import (
	"context"

	"go.mongodb.org/mongo-driver/mongo"
)

// U3P1Rule: "Good morning cadet + Establishing a foothold"
// Trigger: questFinishEvent:17
// Logic: Attempt-based count
//   - Count DialogueNodeEvent:10:30 in window
// Green: count > 1
// Yellow: count <= 1
type U3P1Rule struct {
	BaseRule
}

// NewU3P1Rule creates a new U3P1 rule.
func NewU3P1Rule() *U3P1Rule {
	return &U3P1Rule{
		BaseRule: NewBaseRule(3, 1, "v1", []string{"questFinishEvent:17"}),
	}
}

// Evaluate counts target events and checks threshold.
func (r *U3P1Rule) Evaluate(ctx context.Context, db *mongo.Database, game, playerID string) (Result, error) {
	helper := NewLogDataHelper(db, game)

	// Get the attempt window for this trigger
	triggerKey := "questFinishEvent:17"
	window, err := helper.GetAttemptWindow(ctx, playerID, triggerKey)
	if err != nil {
		return Result{}, err
	}
	if window == nil {
		return Yellow("NO_TRIGGER", map[string]any{
			"reason": "No trigger event found",
		}), nil
	}

	// Count DialogueNodeEvent:10:30 in window
	targetKey := "DialogueNodeEvent:10:30"
	count, err := helper.CountEventInIDWindow(ctx, playerID, targetKey, window)
	if err != nil {
		return Result{}, err
	}

	// Green if count > 1
	if count > 1 {
		return Green(), nil
	}

	return Yellow("LOW_COUNT", map[string]any{
		"count":     count,
		"threshold": 1,
	}), nil
}
