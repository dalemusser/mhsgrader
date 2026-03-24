package rules

import (
	"context"

	"go.mongodb.org/mongo-driver/mongo"
)

// U3P2Rule — Pollution Solution Part I: Capped penalty scoring.
// score = 5 - cappedPenalty(c27) - cappedPenalty(c29+c230); Green if >= 3.
type U3P2Rule struct{ BaseRule }

func NewU3P2Rule() *U3P2Rule {
	return &U3P2Rule{NewBaseRule(3, 2, "v2",
		[]string{"questFinishEvent:17"},
		[]string{"DialogueNodeEvent:11:34"},
	)}
}

// cappedPenalty returns penalty: 0 if cnt <= 1, 1 if cnt <= 3, 2 if cnt >= 4.
func cappedPenalty(cnt int64) int {
	if cnt <= 1 {
		return 0
	}
	if cnt <= 3 {
		return 1
	}
	return 2
}

func (r *U3P2Rule) Evaluate(ctx context.Context, db *mongo.Database, game, playerID string, ec EvalContext) (Result, error) {
	helper := NewLogDataHelper(db, game)
	window := ec.Window

	c27, err := helper.CountEventInIDWindow(ctx, playerID, "DialogueNodeEvent:11:27", window)
	if err != nil {
		return Result{}, err
	}

	c29, err := helper.CountEventInIDWindow(ctx, playerID, "DialogueNodeEvent:11:29", window)
	if err != nil {
		return Result{}, err
	}

	c230, err := helper.CountEventInIDWindow(ctx, playerID, "DialogueNodeEvent:11:230", window)
	if err != nil {
		return Result{}, err
	}

	score := 5 - cappedPenalty(c27) - cappedPenalty(c29+c230)

	totalMistakes := c27 + c29 + c230
	metrics := map[string]any{
		"c27":          c27,
		"c29":          c29,
		"c230":         c230,
		"score":        score,
		"mistakeCount": totalMistakes,
	}
	if score >= 3 {
		return PassedWithMetrics(metrics), nil
	}
	return Flagged("BAD_FEEDBACK", metrics), nil
}
