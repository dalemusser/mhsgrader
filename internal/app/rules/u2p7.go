// internal/app/rules/u2p7.go
package rules

import (
	"context"

	"go.mongodb.org/mongo-driver/mongo"
)

// U2P7Rule: Two triggers -> Success + negative count <= 3
type U2P7Rule struct {
	BaseRule
}

// NewU2P7Rule creates a new U2P7 rule.
func NewU2P7Rule() *U2P7Rule {
	return &U2P7Rule{
		BaseRule: NewBaseRule(2, 7, "v1", []string{
			"QuestFinishEvent:24",
			"DialogueNodeEvent:24:50",
		}),
	}
}

const u2p7NegativeThreshold = 3

// Evaluate checks success and negative event count.
func (r *U2P7Rule) Evaluate(ctx context.Context, db *mongo.Database, game, playerID string) (Result, error) {
	helper := NewLogDataHelper(db, game)

	// Count negative events
	negativeEvents := []string{
		"DialogueNodeEvent:24:30",
		"DialogueNodeEvent:24:35",
		"DialogueNodeEvent:24:40",
	}

	var negativeCount int64
	for _, event := range negativeEvents {
		count, err := helper.CountEvent(ctx, playerID, event)
		if err != nil {
			return Result{}, err
		}
		negativeCount += count
	}

	if negativeCount > u2p7NegativeThreshold {
		return Yellow("TOO_MANY_NEGATIVES", map[string]any{
			"negativeCount": negativeCount,
			"threshold":     u2p7NegativeThreshold,
		}), nil
	}

	return Green(), nil
}
