package rules

import (
	"context"

	"go.mongodb.org/mongo-driver/mongo"
)

// U1P2Rule — Info and Intros: Completion-only.
type U1P2Rule struct{ BaseRule }

func NewU1P2Rule() *U1P2Rule {
	return &U1P2Rule{NewBaseRule(1, 2, "v2",
		[]string{"DialogueNodeEvent:31:29"},
		[]string{"DialogueNodeEvent:30:98"},
	)}
}

func (r *U1P2Rule) Evaluate(ctx context.Context, db *mongo.Database, game, playerID string, ec EvalContext) (Result, error) {
	return PassedWithMetrics(map[string]any{"mistakeCount": int64(0)}), nil
}
