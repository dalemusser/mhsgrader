// internal/app/rules/u1p1.go
package rules

import (
	"context"

	"go.mongodb.org/mongo-driver/mongo"
)

// U1P1Rule: Trigger DialogueNodeEvent:31:29 -> Always green
type U1P1Rule struct {
	BaseRule
}

// NewU1P1Rule creates a new U1P1 rule.
func NewU1P1Rule() *U1P1Rule {
	return &U1P1Rule{
		BaseRule: NewBaseRule(1, 1, "v1", []string{"DialogueNodeEvent:31:29"}),
	}
}

// Evaluate always returns green for this rule.
func (r *U1P1Rule) Evaluate(ctx context.Context, db *mongo.Database, game, playerID string) (Result, error) {
	return Green(), nil
}
