// internal/app/rules/u2p1.go
package rules

import (
	"context"

	"go.mongodb.org/mongo-driver/mongo"
)

// U2P1Rule: "Escape the Ruins + Topographic Glyph"
// Trigger: questFinishEvent:21
// Logic: Attempt-based
//   - Must have success key DialogueNodeEvent:68:29 in window
//   - Must NOT have any yellow nodes in window
// Yellow nodes: 68:23, 68:27, 68:28, 68:31
// Green: hasSuccess && !hasAnyYellow
// Yellow: otherwise
type U2P1Rule struct {
	BaseRule
}

// NewU2P1Rule creates a new U2P1 rule.
func NewU2P1Rule() *U2P1Rule {
	return &U2P1Rule{
		BaseRule: NewBaseRule(2, 1, "v1", []string{"questFinishEvent:21"}),
	}
}

// Evaluate checks for success conditions within the attempt window.
func (r *U2P1Rule) Evaluate(ctx context.Context, db *mongo.Database, game, playerID string) (Result, error) {
	helper := NewLogDataHelper(db, game)

	// Get the attempt window for this trigger
	triggerKey := "questFinishEvent:21"
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
	successKey := "DialogueNodeEvent:68:29"
	hasSuccess, err := helper.HasEventInWindow(ctx, playerID, successKey, window)
	if err != nil {
		return Result{}, err
	}

	// Check for yellow nodes in window
	yellowNodes := []string{
		"DialogueNodeEvent:68:23",
		"DialogueNodeEvent:68:27",
		"DialogueNodeEvent:68:28",
		"DialogueNodeEvent:68:31",
	}

	hasYellowNode, err := helper.HasAnyEventInWindow(ctx, playerID, yellowNodes, window)
	if err != nil {
		return Result{}, err
	}

	// Green if hasSuccess and no yellow nodes
	if hasSuccess && !hasYellowNode {
		return Green(), nil
	}

	return Yellow("INCORRECT_PATH", map[string]any{
		"hasSuccess":    hasSuccess,
		"hasYellowNode": hasYellowNode,
	}), nil
}
