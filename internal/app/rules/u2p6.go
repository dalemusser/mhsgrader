// internal/app/rules/u2p6.go
package rules

import (
	"context"

	"go.mongodb.org/mongo-driver/mongo"
)

// U2P6Rule: "Drone Tutorial + Data Collection"
// Trigger: DialogueNodeEvent:20:46
// Logic: Attempt-based
//   - Must have pass key DialogueNodeEvent:20:43 in window
//   - Must NOT have yellow keys in window
// Yellow keys: 20:44, 20:45
// Green: hasPass && !hasYellow
// Yellow: !hasPass || hasYellow
type U2P6Rule struct {
	BaseRule
}

// NewU2P6Rule creates a new U2P6 rule.
func NewU2P6Rule() *U2P6Rule {
	return &U2P6Rule{
		BaseRule: NewBaseRule(2, 6, "v1", []string{"DialogueNodeEvent:20:46"}),
	}
}

// Evaluate checks for pass and absence of yellow nodes.
func (r *U2P6Rule) Evaluate(ctx context.Context, db *mongo.Database, game, playerID string) (Result, error) {
	helper := NewLogDataHelper(db, game)

	// Get the attempt window for this trigger
	triggerKey := "DialogueNodeEvent:20:46"
	window, err := helper.GetAttemptWindow(ctx, playerID, triggerKey)
	if err != nil {
		return Result{}, err
	}
	if window == nil {
		return Yellow("NO_TRIGGER", map[string]any{
			"reason": "No trigger event found",
		}), nil
	}

	// Check for pass key in window
	passKey := "DialogueNodeEvent:20:43"
	hasPass, err := helper.HasEventInWindow(ctx, playerID, passKey, window)
	if err != nil {
		return Result{}, err
	}

	// Check for yellow nodes in window
	yellowNodes := []string{
		"DialogueNodeEvent:20:44",
		"DialogueNodeEvent:20:45",
	}

	hasYellowNode, err := helper.HasAnyEventInWindow(ctx, playerID, yellowNodes, window)
	if err != nil {
		return Result{}, err
	}

	// Green if hasPass and no yellow nodes
	if hasPass && !hasYellowNode {
		return Green(), nil
	}

	return Yellow("YELLOW_PATH", map[string]any{
		"hasPass":       hasPass,
		"hasYellowNode": hasYellowNode,
	}), nil
}
