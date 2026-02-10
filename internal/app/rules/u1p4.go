// internal/app/rules/u1p4.go
package rules

import (
	"context"

	"go.mongodb.org/mongo-driver/mongo"
)

// U1P4Rule: Trigger QuestFinishEvent:34 -> Always green
type U1P4Rule struct {
	BaseRule
}

// NewU1P4Rule creates a new U1P4 rule.
func NewU1P4Rule() *U1P4Rule {
	return &U1P4Rule{
		BaseRule: NewBaseRule(1, 4, "v1", []string{"QuestFinishEvent:34"}),
	}
}

// Evaluate always returns green for this rule.
func (r *U1P4Rule) Evaluate(ctx context.Context, db *mongo.Database, game, playerID string) (Result, error) {
	return Green(), nil
}
