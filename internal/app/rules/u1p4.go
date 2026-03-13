package rules

import (
	"context"

	"go.mongodb.org/mongo-driver/mongo"
)

// U1P4Rule — What Was That?: Completion-only.
type U1P4Rule struct{ BaseRule }

func NewU1P4Rule() *U1P4Rule {
	return &U1P4Rule{NewBaseRule(1, 4, "v2",
		[]string{"questActiveEvent:34"},
		[]string{"DialogueNodeEvent:33:19"},
	)}
}

func (r *U1P4Rule) Evaluate(ctx context.Context, db *mongo.Database, game, playerID string) (Result, error) {
	return Passed(), nil
}
