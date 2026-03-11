// internal/app/rules/u2p5.go
package rules

import (
	"context"

	"go.mongodb.org/mongo-driver/mongo"
)

// U2P5Rule: "Getting the Band Back Together"
// Trigger: DialogueNodeEvent:23:42
// Logic: Attempt-based scoring
//   - Count POS_KEYS (22 keys: 26:165 through 26:186)
//   - Count NEG_KEYS (25 keys: 26:187 through 26:211)
//   - score = posCount - (negCount / 3.0)
// Green: score >= 4
// Yellow: score < 4
type U2P5Rule struct {
	BaseRule
}

// NewU2P5Rule creates a new U2P5 rule.
func NewU2P5Rule() *U2P5Rule {
	return &U2P5Rule{
		BaseRule: NewBaseRule(2, 5, "v1", []string{"DialogueNodeEvent:23:42"}),
	}
}

const u2p5Threshold = 4.0

// Evaluate calculates score and checks against threshold.
func (r *U2P5Rule) Evaluate(ctx context.Context, db *mongo.Database, game, playerID string) (Result, error) {
	helper := NewLogDataHelper(db, game)

	// Get the attempt window for this trigger
	triggerKey := "DialogueNodeEvent:23:42"
	window, err := helper.GetAttemptWindow(ctx, playerID, triggerKey)
	if err != nil {
		return Result{}, err
	}
	if window == nil {
		return Yellow("NO_TRIGGER", map[string]any{
			"reason": "No trigger event found",
		}), nil
	}

	// Positive keys: 26:165 through 26:186 (22 keys)
	posKeys := []string{
		"DialogueNodeEvent:26:165",
		"DialogueNodeEvent:26:166",
		"DialogueNodeEvent:26:167",
		"DialogueNodeEvent:26:168",
		"DialogueNodeEvent:26:169",
		"DialogueNodeEvent:26:170",
		"DialogueNodeEvent:26:171",
		"DialogueNodeEvent:26:172",
		"DialogueNodeEvent:26:173",
		"DialogueNodeEvent:26:174",
		"DialogueNodeEvent:26:175",
		"DialogueNodeEvent:26:176",
		"DialogueNodeEvent:26:177",
		"DialogueNodeEvent:26:178",
		"DialogueNodeEvent:26:179",
		"DialogueNodeEvent:26:180",
		"DialogueNodeEvent:26:181",
		"DialogueNodeEvent:26:182",
		"DialogueNodeEvent:26:183",
		"DialogueNodeEvent:26:184",
		"DialogueNodeEvent:26:185",
		"DialogueNodeEvent:26:186",
	}

	// Negative keys: 26:187 through 26:211 (25 keys)
	negKeys := []string{
		"DialogueNodeEvent:26:187",
		"DialogueNodeEvent:26:188",
		"DialogueNodeEvent:26:189",
		"DialogueNodeEvent:26:190",
		"DialogueNodeEvent:26:191",
		"DialogueNodeEvent:26:192",
		"DialogueNodeEvent:26:193",
		"DialogueNodeEvent:26:194",
		"DialogueNodeEvent:26:195",
		"DialogueNodeEvent:26:196",
		"DialogueNodeEvent:26:197",
		"DialogueNodeEvent:26:198",
		"DialogueNodeEvent:26:199",
		"DialogueNodeEvent:26:200",
		"DialogueNodeEvent:26:201",
		"DialogueNodeEvent:26:202",
		"DialogueNodeEvent:26:203",
		"DialogueNodeEvent:26:204",
		"DialogueNodeEvent:26:205",
		"DialogueNodeEvent:26:206",
		"DialogueNodeEvent:26:207",
		"DialogueNodeEvent:26:208",
		"DialogueNodeEvent:26:209",
		"DialogueNodeEvent:26:210",
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

	// score = posCount - (negCount / 3.0)
	score := float64(posCount) - (float64(negCount) / 3.0)

	if score < u2p5Threshold {
		return Yellow("LOW_SCORE", map[string]any{
			"score":     score,
			"posCount":  posCount,
			"negCount":  negCount,
			"threshold": u2p5Threshold,
		}), nil
	}

	return Green(), nil
}
