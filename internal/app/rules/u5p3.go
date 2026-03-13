package rules

import (
	"context"

	"go.mongodb.org/mongo-driver/mongo"
)

// U5P3Rule — What Happened Here?: Count negative dialogues.
// Green if count < 4; Flagged if >= 4.
type U5P3Rule struct{ BaseRule }

func NewU5P3Rule() *U5P3Rule {
	return &U5P3Rule{NewBaseRule(5, 3, "v2",
		[]string{"DialogueNodeEvent:96:1"},
		[]string{"questFinishEvent:44"},
	)}
}

func (r *U5P3Rule) Evaluate(ctx context.Context, db *mongo.Database, game, playerID string) (Result, error) {
	helper := NewLogDataHelper(db, game)

	window, err := helper.GetAttemptWindow(ctx, playerID, "questFinishEvent:44")
	if err != nil {
		return Result{}, err
	}
	if window == nil {
		return Flagged("NO_TRIGGER", nil), nil
	}

	negKeys := []string{
		"DialogueNodeEvent:108:25", "DialogueNodeEvent:108:32", "DialogueNodeEvent:108:33",
		"DialogueNodeEvent:108:37", "DialogueNodeEvent:108:39", "DialogueNodeEvent:108:41",
		"DialogueNodeEvent:108:47", "DialogueNodeEvent:108:53", "DialogueNodeEvent:108:54",
		"DialogueNodeEvent:108:55", "DialogueNodeEvent:108:59", "DialogueNodeEvent:108:60",
		"DialogueNodeEvent:108:61", "DialogueNodeEvent:108:62", "DialogueNodeEvent:108:70",
		"DialogueNodeEvent:108:72", "DialogueNodeEvent:108:73", "DialogueNodeEvent:108:74",
		"DialogueNodeEvent:108:75", "DialogueNodeEvent:108:76", "DialogueNodeEvent:108:78",
		"DialogueNodeEvent:108:79", "DialogueNodeEvent:108:80", "DialogueNodeEvent:108:82",
		"DialogueNodeEvent:108:83", "DialogueNodeEvent:108:84", "DialogueNodeEvent:108:85",
		"DialogueNodeEvent:108:86", "DialogueNodeEvent:108:87", "DialogueNodeEvent:108:88",
		"DialogueNodeEvent:108:89", "DialogueNodeEvent:108:90", "DialogueNodeEvent:108:91",
	}

	count, err := helper.CountEventsInWindow(ctx, playerID, negKeys, window)
	if err != nil {
		return Result{}, err
	}

	if count < 4 {
		return Passed(), nil
	}
	return Flagged("TOO_MANY_NEGATIVES", map[string]any{"negativeCount": count}), nil
}
