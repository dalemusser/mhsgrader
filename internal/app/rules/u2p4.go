// internal/app/rules/u2p4.go
package rules

import (
	"context"

	"go.mongodb.org/mongo-driver/mongo"
)

// U2P4Rule: Trigger DialogueNodeEvent:23:17 -> Success exists + no bad feedback
type U2P4Rule struct {
	BaseRule
}

// NewU2P4Rule creates a new U2P4 rule.
func NewU2P4Rule() *U2P4Rule {
	return &U2P4Rule{
		BaseRule: NewBaseRule(2, 4, "v1", []string{"DialogueNodeEvent:23:17"}),
	}
}

// Evaluate checks for success and absence of bad feedback.
func (r *U2P4Rule) Evaluate(ctx context.Context, db *mongo.Database, game, playerID string) (Result, error) {
	helper := NewLogDataHelper(db, game)

	// Check for bad feedback nodes
	badFeedbackNodes := []string{
		"DialogueNodeEvent:23:20",
		"DialogueNodeEvent:23:25",
	}

	hasBadFeedback, err := helper.HasAnyEvent(ctx, playerID, badFeedbackNodes)
	if err != nil {
		return Result{}, err
	}

	if hasBadFeedback {
		return Yellow("BAD_FEEDBACK", map[string]any{
			"reason": "Player received negative feedback",
		}), nil
	}

	return Green(), nil
}
