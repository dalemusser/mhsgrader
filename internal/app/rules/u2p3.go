// internal/app/rules/u2p3.go
package rules

import (
	"context"

	"go.mongodb.org/mongo-driver/mongo"
)

// U2P3Rule: "Finding Tera & Aryn"
// Trigger: DialogueNodeEvent:22:18
// Logic: Attempt-based with START/END window
//   - END = latest trigger DialogueNodeEvent:22:18
//   - START = latest DialogueNodeEvent:20:33 before END
//   - Count TARGET_KEYS in window
// TARGET_KEYS: 33 keys (see targetKeys list below)
// Green: count <= 6
// Yellow: count > 6
type U2P3Rule struct {
	BaseRule
}

// NewU2P3Rule creates a new U2P3 rule.
func NewU2P3Rule() *U2P3Rule {
	return &U2P3Rule{
		BaseRule: NewBaseRule(2, 3, "v1", []string{"DialogueNodeEvent:22:18"}),
	}
}

const u2p3Threshold = 6

// Evaluate checks target count against threshold.
func (r *U2P3Rule) Evaluate(ctx context.Context, db *mongo.Database, game, playerID string) (Result, error) {
	helper := NewLogDataHelper(db, game)

	// Get the window between DialogueNodeEvent:20:33 (START) and DialogueNodeEvent:22:18 (END)
	startKey := "DialogueNodeEvent:20:33"
	endKey := "DialogueNodeEvent:22:18"
	window, err := helper.GetWindowBetweenEvents(ctx, playerID, startKey, endKey)
	if err != nil {
		return Result{}, err
	}
	if window == nil {
		return Yellow("NO_WINDOW", map[string]any{
			"reason": "Missing start or end event for window",
		}), nil
	}

	// Count target keys in window (33 keys from the rules document)
	targetKeys := []string{
		"DialogueNodeEvent:22:6",
		"DialogueNodeEvent:22:7",
		"DialogueNodeEvent:22:8",
		"DialogueNodeEvent:22:9",
		"DialogueNodeEvent:22:10",
		"DialogueNodeEvent:22:11",
		"DialogueNodeEvent:22:12",
		"DialogueNodeEvent:22:13",
		"DialogueNodeEvent:22:14",
		"DialogueNodeEvent:22:15",
		"DialogueNodeEvent:22:16",
		"DialogueNodeEvent:22:17",
		"DialogueNodeEvent:22:19",
		"DialogueNodeEvent:22:20",
		"DialogueNodeEvent:22:21",
		"DialogueNodeEvent:22:22",
		"DialogueNodeEvent:22:23",
		"DialogueNodeEvent:22:24",
		"DialogueNodeEvent:22:25",
		"DialogueNodeEvent:22:26",
		"DialogueNodeEvent:22:27",
		"DialogueNodeEvent:22:28",
		"DialogueNodeEvent:22:29",
		"DialogueNodeEvent:22:30",
		"DialogueNodeEvent:22:31",
		"DialogueNodeEvent:22:32",
		"DialogueNodeEvent:22:33",
		"DialogueNodeEvent:22:34",
		"DialogueNodeEvent:22:35",
		"DialogueNodeEvent:22:36",
		"DialogueNodeEvent:22:37",
		"DialogueNodeEvent:22:38",
		"DialogueNodeEvent:22:39",
	}

	count, err := helper.CountEventsInWindow(ctx, playerID, targetKeys, window)
	if err != nil {
		return Result{}, err
	}

	if count > u2p3Threshold {
		return Yellow("TOO_MANY_TARGETS", map[string]any{
			"countTargets": count,
			"threshold":    u2p3Threshold,
		}), nil
	}

	return Green(), nil
}
