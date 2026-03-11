// internal/app/rules/u2p2.go
package rules

import (
	"context"

	"go.mongodb.org/mongo-driver/mongo"
)

// U2P2Rule: "Foraged Forging + Finding Toppo"
// Trigger: DialogueNodeEvent:20:26
// Logic: Attempt-based with START/END window
//   - END = latest trigger DialogueNodeEvent:20:26
//   - START = latest questFinishEvent:21 before END
//   - Count TARGET_KEYS in window
// TARGET_KEYS (9 keys): 18:99, 28:179, 59:179, 18:223, 28:182, 59:182, 18:224, 28:183, 59:183
// Green: count <= 1
// Yellow: count > 1
type U2P2Rule struct {
	BaseRule
}

// NewU2P2Rule creates a new U2P2 rule.
func NewU2P2Rule() *U2P2Rule {
	return &U2P2Rule{
		BaseRule: NewBaseRule(2, 2, "v1", []string{"DialogueNodeEvent:20:26"}),
	}
}

const u2p2Threshold = 1

// Evaluate checks attempt count against threshold.
func (r *U2P2Rule) Evaluate(ctx context.Context, db *mongo.Database, game, playerID string) (Result, error) {
	helper := NewLogDataHelper(db, game)

	// Get the window between questFinishEvent:21 (START) and DialogueNodeEvent:20:26 (END)
	startKey := "questFinishEvent:21"
	endKey := "DialogueNodeEvent:20:26"
	window, err := helper.GetWindowBetweenEvents(ctx, playerID, startKey, endKey)
	if err != nil {
		return Result{}, err
	}
	if window == nil {
		return Yellow("NO_WINDOW", map[string]any{
			"reason": "Missing start or end event for window",
		}), nil
	}

	// Count target keys in window
	targetKeys := []string{
		"DialogueNodeEvent:18:99",
		"DialogueNodeEvent:28:179",
		"DialogueNodeEvent:59:179",
		"DialogueNodeEvent:18:223",
		"DialogueNodeEvent:28:182",
		"DialogueNodeEvent:59:182",
		"DialogueNodeEvent:18:224",
		"DialogueNodeEvent:28:183",
		"DialogueNodeEvent:59:183",
	}

	count, err := helper.CountEventsInWindow(ctx, playerID, targetKeys, window)
	if err != nil {
		return Result{}, err
	}

	if count > u2p2Threshold {
		return Yellow("TOO_MANY_ATTEMPTS", map[string]any{
			"count":     count,
			"threshold": u2p2Threshold,
		}), nil
	}

	return Green(), nil
}
