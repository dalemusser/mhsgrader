// internal/app/rules/u2p2.go
package rules

import (
	"context"

	"go.mongodb.org/mongo-driver/mongo"
)

// U2P2Rule: Trigger DialogueNodeEvent:20:26 -> Windowed count (threshold: 1)
// Yellow if count of attempts > 1 in current window
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

	// Window starts at QuestActiveEvent:20
	windowStartEvent := "QuestActiveEvent:20"
	countEvent := "DialogueNodeEvent:20:26"

	count, err := helper.CountEventInWindow(ctx, playerID, windowStartEvent, countEvent)
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
