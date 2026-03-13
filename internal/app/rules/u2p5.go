package rules

import (
	"context"

	"go.mongodb.org/mongo-driver/mongo"
)

// U2P5Rule — Classified Information: Weighted pos/neg scoring.
// score = posCount - (negCount / 3.0); Green if >= 4.
type U2P5Rule struct{ BaseRule }

func NewU2P5Rule() *U2P5Rule {
	return &U2P5Rule{NewBaseRule(2, 5, "v2",
		[]string{"DialogueNodeEvent:23:17"},
		[]string{"DialogueNodeEvent:23:42"},
	)}
}

func (r *U2P5Rule) Evaluate(ctx context.Context, db *mongo.Database, game, playerID string) (Result, error) {
	helper := NewLogDataHelper(db, game)

	window, err := helper.GetAttemptWindow(ctx, playerID, "DialogueNodeEvent:23:42")
	if err != nil {
		return Result{}, err
	}
	if window == nil {
		return Flagged("NO_TRIGGER", nil), nil
	}

	posKeys := []string{
		"DialogueNodeEvent:26:165", "DialogueNodeEvent:26:166", "DialogueNodeEvent:26:167",
		"DialogueNodeEvent:26:168", "DialogueNodeEvent:26:169", "DialogueNodeEvent:26:170",
		"DialogueNodeEvent:26:171", "DialogueNodeEvent:26:172", "DialogueNodeEvent:26:173",
		"DialogueNodeEvent:26:174", "DialogueNodeEvent:26:175", "DialogueNodeEvent:26:176",
		"DialogueNodeEvent:26:177", "DialogueNodeEvent:26:178", "DialogueNodeEvent:26:179",
		"DialogueNodeEvent:26:180", "DialogueNodeEvent:26:181", "DialogueNodeEvent:26:182",
		"DialogueNodeEvent:26:183", "DialogueNodeEvent:26:184", "DialogueNodeEvent:26:185",
		"DialogueNodeEvent:26:186",
	}

	negKeys := []string{
		"DialogueNodeEvent:26:187", "DialogueNodeEvent:26:188", "DialogueNodeEvent:26:189",
		"DialogueNodeEvent:26:190", "DialogueNodeEvent:26:191", "DialogueNodeEvent:26:192",
		"DialogueNodeEvent:26:193", "DialogueNodeEvent:26:194", "DialogueNodeEvent:26:195",
		"DialogueNodeEvent:26:196", "DialogueNodeEvent:26:197", "DialogueNodeEvent:26:198",
		"DialogueNodeEvent:26:199", "DialogueNodeEvent:26:200", "DialogueNodeEvent:26:201",
		"DialogueNodeEvent:26:202", "DialogueNodeEvent:26:203", "DialogueNodeEvent:26:204",
		"DialogueNodeEvent:26:205", "DialogueNodeEvent:26:206", "DialogueNodeEvent:26:207",
		"DialogueNodeEvent:26:208", "DialogueNodeEvent:26:209", "DialogueNodeEvent:26:210",
		"DialogueNodeEvent:26:211",
	}

	posCount, err := helper.CountEventsInWindow(ctx, playerID, posKeys, window)
	if err != nil {
		return Result{}, err
	}

	negCount, err := helper.CountEventsInWindow(ctx, playerID, negKeys, window)
	if err != nil {
		return Result{}, err
	}

	score := float64(posCount) - (float64(negCount) / 3.0)

	if score >= 4.0 {
		return Passed(), nil
	}
	return Flagged("TOO_MANY_NEGATIVES", map[string]any{
		"score": score, "posCount": posCount, "negCount": negCount, "wrong_number": negCount,
	}), nil
}
