package rules

import (
	"context"

	"go.mongodb.org/mongo-driver/mongo"
)

// U1P1Rule — Getting Your Space Legs: Completion-only.
type U1P1Rule struct{ BaseRule }

func NewU1P1Rule() *U1P1Rule {
	return &U1P1Rule{NewBaseRule(1, 1, "v2",
		[]string{"questActiveEvent:28"},
		[]string{"DialogueNodeEvent:31:29"},
	)}
}

func (r *U1P1Rule) Evaluate(ctx context.Context, db *mongo.Database, game, playerID string) (Result, error) {
	return Passed(), nil
}
