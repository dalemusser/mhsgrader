// internal/app/rules/u3p4.go
package rules

import (
	"context"

	"go.mongodb.org/mongo-driver/mongo"
)

// U3P4Rule: "Forsaken Facility"
// Trigger: DialogueNodeEvent:73:200
// Logic: Attempt-based with gate
//   - Gate: must have DialogueNodeEvent:78:24 in window (else yellow)
//   - Count TARGET_KEYS (8 keys: 78:3,4,7,9,10,12,18,23)
//   - score: 2 if count == 0, 1 if count <= 2, 0 if count >= 3
// Green: score > 0 (i.e., count <= 2)
// Yellow: score == 0 (count >= 3) or gate failed
type U3P4Rule struct {
	BaseRule
}

// NewU3P4Rule creates a new U3P4 rule.
func NewU3P4Rule() *U3P4Rule {
	return &U3P4Rule{
		BaseRule: NewBaseRule(3, 4, "v1", []string{"DialogueNodeEvent:73:200"}),
	}
}

// Evaluate checks gate and calculates score.
func (r *U3P4Rule) Evaluate(ctx context.Context, db *mongo.Database, game, playerID string) (Result, error) {
	helper := NewLogDataHelper(db, game)

	// Get the attempt window for this trigger
	triggerKey := "DialogueNodeEvent:73:200"
	window, err := helper.GetAttemptWindow(ctx, playerID, triggerKey)
	if err != nil {
		return Result{}, err
	}
	if window == nil {
		return Yellow("NO_TRIGGER", map[string]any{
			"reason": "No trigger event found",
		}), nil
	}

	// Gate: must have DialogueNodeEvent:78:24 in window
	gateKey := "DialogueNodeEvent:78:24"
	hasGate, err := helper.HasEventInWindow(ctx, playerID, gateKey, window)
	if err != nil {
		return Result{}, err
	}

	if !hasGate {
		return Yellow("GATE_FAILED", map[string]any{
			"reason": "Missing required gate event 78:24",
		}), nil
	}

	// Target keys: 78:3,4,7,9,10,12,18,23 (8 keys)
	targetKeys := []string{
		"DialogueNodeEvent:78:3",
		"DialogueNodeEvent:78:4",
		"DialogueNodeEvent:78:7",
		"DialogueNodeEvent:78:9",
		"DialogueNodeEvent:78:10",
		"DialogueNodeEvent:78:12",
		"DialogueNodeEvent:78:18",
		"DialogueNodeEvent:78:23",
	}

	count, err := helper.CountEventsInWindow(ctx, playerID, targetKeys, window)
	if err != nil {
		return Result{}, err
	}

	// Calculate score: 2 if count == 0, 1 if count <= 2, 0 if count >= 3
	var score int
	if count == 0 {
		score = 2
	} else if count <= 2 {
		score = 1
	} else {
		score = 0
	}

	// Green if score > 0 (count <= 2)
	if score > 0 {
		return Green(), nil
	}

	return Yellow("TOO_MANY_ERRORS", map[string]any{
		"count": count,
		"score": score,
	}), nil
}
