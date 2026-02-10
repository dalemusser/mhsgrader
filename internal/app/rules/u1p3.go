// internal/app/rules/u1p3.go
package rules

import (
	"context"

	"go.mongodb.org/mongo-driver/mongo"
)

// U1P3Rule: Trigger QuestActiveEvent:34 -> Yellow if certain nodes exist
// Yellow if player has DialogueNodeEvent:34:5 or DialogueNodeEvent:34:6
type U1P3Rule struct {
	BaseRule
}

// NewU1P3Rule creates a new U1P3 rule.
func NewU1P3Rule() *U1P3Rule {
	return &U1P3Rule{
		BaseRule: NewBaseRule(1, 3, "v1", []string{"QuestActiveEvent:34"}),
	}
}

// Evaluate checks for yellow conditions.
func (r *U1P3Rule) Evaluate(ctx context.Context, db *mongo.Database, game, playerID string) (Result, error) {
	helper := NewLogDataHelper(db, game)

	// Check for yellow nodes
	yellowNodes := []string{
		"DialogueNodeEvent:34:5",
		"DialogueNodeEvent:34:6",
	}

	hasYellowNode, err := helper.HasAnyEvent(ctx, playerID, yellowNodes)
	if err != nil {
		return Result{}, err
	}

	if hasYellowNode {
		return Yellow("INCORRECT_PATH", map[string]any{
			"reason": "Player took incorrect dialogue path",
		}), nil
	}

	return Green(), nil
}
