// internal/app/rules/u2p1.go
package rules

import (
	"context"

	"go.mongodb.org/mongo-driver/mongo"
)

// U2P1Rule: Trigger QuestFinishEvent:21 -> Check for success + no yellow nodes
type U2P1Rule struct {
	BaseRule
}

// NewU2P1Rule creates a new U2P1 rule.
func NewU2P1Rule() *U2P1Rule {
	return &U2P1Rule{
		BaseRule: NewBaseRule(2, 1, "v1", []string{"QuestFinishEvent:21"}),
	}
}

// Evaluate checks for success conditions.
func (r *U2P1Rule) Evaluate(ctx context.Context, db *mongo.Database, game, playerID string) (Result, error) {
	helper := NewLogDataHelper(db, game)

	// Check for yellow nodes indicating incorrect path
	yellowNodes := []string{
		"DialogueNodeEvent:21:10",
		"DialogueNodeEvent:21:15",
	}

	hasYellowNode, err := helper.HasAnyEvent(ctx, playerID, yellowNodes)
	if err != nil {
		return Result{}, err
	}

	if hasYellowNode {
		return Yellow("INCORRECT_PATH", map[string]any{
			"reason": "Player took incorrect path in quest 21",
		}), nil
	}

	return Green(), nil
}
