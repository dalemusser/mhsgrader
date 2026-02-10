// internal/app/rules/u2p3.go
package rules

import (
	"context"

	"go.mongodb.org/mongo-driver/mongo"
)

// U2P3Rule: Trigger DialogueNodeEvent:22:18 -> Windowed count (threshold: 6)
// Yellow if target count > threshold
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

	// Window starts at QuestActiveEvent:22
	windowStartEvent := "QuestActiveEvent:22"
	countEvent := "DialogueNodeEvent:22:18"

	count, err := helper.CountEventInWindow(ctx, playerID, windowStartEvent, countEvent)
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
