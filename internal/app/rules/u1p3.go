// internal/app/rules/u1p3.go
package rules

import (
	"context"

	"go.mongodb.org/mongo-driver/mongo"
)

// U1P3Rule: "Defend the Expedition"
// Trigger: questActiveEvent:34
// Logic: Attempt-based - yellow if any yellow node in window
// Yellow nodes: DialogueNodeEvent:70:25, DialogueNodeEvent:70:33
type U1P3Rule struct {
	BaseRule
}

// NewU1P3Rule creates a new U1P3 rule.
func NewU1P3Rule() *U1P3Rule {
	return &U1P3Rule{
		BaseRule: NewBaseRule(1, 3, "v1", []string{"questActiveEvent:34"}),
	}
}

// Evaluate checks for yellow conditions within the attempt window.
func (r *U1P3Rule) Evaluate(ctx context.Context, db *mongo.Database, game, playerID string) (Result, error) {
	helper := NewLogDataHelper(db, game)

	// Get the attempt window for this trigger
	triggerKey := "questActiveEvent:34"
	window, err := helper.GetAttemptWindow(ctx, playerID, triggerKey)
	if err != nil {
		return Result{}, err
	}
	if window == nil {
		// No trigger found - return yellow
		return Yellow("NO_TRIGGER", map[string]any{
			"reason": "No trigger event found",
		}), nil
	}

	// Check for yellow nodes in the window
	yellowNodes := []string{
		"DialogueNodeEvent:70:25",
		"DialogueNodeEvent:70:33",
	}

	hasYellowNode, err := helper.HasAnyEventInWindow(ctx, playerID, yellowNodes, window)
	if err != nil {
		return Result{}, err
	}

	if hasYellowNode {
		return Yellow("INCORRECT_PATH", map[string]any{
			"reason": "Player took incorrect dialogue path",
		}), nil
	}

	return Green(), nil
}
