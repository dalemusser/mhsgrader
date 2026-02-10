// internal/app/rules/u2p5.go
package rules

import (
	"context"

	"go.mongodb.org/mongo-driver/mongo"
)

// U2P5Rule: Trigger DialogueNodeEvent:23:42 -> Score calculation (threshold: 4)
type U2P5Rule struct {
	BaseRule
}

// NewU2P5Rule creates a new U2P5 rule.
func NewU2P5Rule() *U2P5Rule {
	return &U2P5Rule{
		BaseRule: NewBaseRule(2, 5, "v1", []string{"DialogueNodeEvent:23:42"}),
	}
}

const u2p5Threshold = 4

// Evaluate calculates score and checks against threshold.
func (r *U2P5Rule) Evaluate(ctx context.Context, db *mongo.Database, game, playerID string) (Result, error) {
	helper := NewLogDataHelper(db, game)

	// Count positive events
	positiveEvents := []string{
		"DialogueNodeEvent:23:10",
		"DialogueNodeEvent:23:15",
		"DialogueNodeEvent:23:18",
	}

	var score int64
	for _, event := range positiveEvents {
		count, err := helper.CountEvent(ctx, playerID, event)
		if err != nil {
			return Result{}, err
		}
		score += count
	}

	if score < u2p5Threshold {
		return Yellow("LOW_SCORE", map[string]any{
			"score":     score,
			"threshold": u2p5Threshold,
		}), nil
	}

	return Green(), nil
}
