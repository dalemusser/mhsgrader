// internal/app/rules/u2p4.go
package rules

import (
	"context"

	"go.mongodb.org/mongo-driver/mongo"
)

// U2P4Rule: "Investigate the Temple & Watershed Glyph"
// Trigger: DialogueNodeEvent:23:17
// Logic: Attempt-based
//   - Must have success DialogueNodeEvent:74:21 in window
//   - Must NOT have any bad keys in window
// Bad keys: 74:16, 74:17, 74:20, 74:22
// Green: hasSuccess && !hasBad
// Yellow: otherwise
type U2P4Rule struct {
	BaseRule
}

// NewU2P4Rule creates a new U2P4 rule.
func NewU2P4Rule() *U2P4Rule {
	return &U2P4Rule{
		BaseRule: NewBaseRule(2, 4, "v1", []string{"DialogueNodeEvent:23:17"}),
	}
}

// Evaluate checks for success and absence of bad feedback.
func (r *U2P4Rule) Evaluate(ctx context.Context, db *mongo.Database, game, playerID string) (Result, error) {
	helper := NewLogDataHelper(db, game)

	// Get the attempt window for this trigger
	triggerKey := "DialogueNodeEvent:23:17"
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
	successKey := "DialogueNodeEvent:74:21"
	hasSuccess, err := helper.HasEventInWindow(ctx, playerID, successKey, window)
	if err != nil {
		return Result{}, err
	}

	// Check for bad feedback nodes in window
	badKeys := []string{
		"DialogueNodeEvent:74:16",
		"DialogueNodeEvent:74:17",
		"DialogueNodeEvent:74:20",
		"DialogueNodeEvent:74:22",
	}

	hasBadFeedback, err := helper.HasAnyEventInWindow(ctx, playerID, badKeys, window)
	if err != nil {
		return Result{}, err
	}

	// Green if hasSuccess and no bad feedback
	if hasSuccess && !hasBadFeedback {
		return Green(), nil
	}

	return Yellow("BAD_FEEDBACK", map[string]any{
		"hasSuccess":    hasSuccess,
		"hasBadFeedback": hasBadFeedback,
	}), nil
}
