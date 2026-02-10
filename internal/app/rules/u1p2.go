// internal/app/rules/u1p2.go
package rules

import (
	"context"

	"go.mongodb.org/mongo-driver/mongo"
)

// U1P2Rule: Trigger DialogueNodeEvent:30:98 -> Always green
type U1P2Rule struct {
	BaseRule
}

// NewU1P2Rule creates a new U1P2 rule.
func NewU1P2Rule() *U1P2Rule {
	return &U1P2Rule{
		BaseRule: NewBaseRule(1, 2, "v1", []string{"DialogueNodeEvent:30:98"}),
	}
}

// Evaluate always returns green for this rule.
func (r *U1P2Rule) Evaluate(ctx context.Context, db *mongo.Database, game, playerID string) (Result, error) {
	return Green(), nil
}
