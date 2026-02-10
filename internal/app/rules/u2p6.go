// internal/app/rules/u2p6.go
package rules

import (
	"context"

	"go.mongodb.org/mongo-driver/mongo"
)

// U2P6Rule: Trigger DialogueNodeEvent:18:284 -> Pass node + no yellow nodes
type U2P6Rule struct {
	BaseRule
}

// NewU2P6Rule creates a new U2P6 rule.
func NewU2P6Rule() *U2P6Rule {
	return &U2P6Rule{
		BaseRule: NewBaseRule(2, 6, "v1", []string{"DialogueNodeEvent:18:284"}),
	}
}

// Evaluate checks for pass and absence of yellow nodes.
func (r *U2P6Rule) Evaluate(ctx context.Context, db *mongo.Database, game, playerID string) (Result, error) {
	helper := NewLogDataHelper(db, game)

	// Check for yellow nodes
	yellowNodes := []string{
		"DialogueNodeEvent:18:100",
		"DialogueNodeEvent:18:150",
	}

	hasYellowNode, err := helper.HasAnyEvent(ctx, playerID, yellowNodes)
	if err != nil {
		return Result{}, err
	}

	if hasYellowNode {
		return Yellow("YELLOW_PATH", map[string]any{
			"reason": "Player encountered yellow path nodes",
		}), nil
	}

	return Green(), nil
}
