// internal/app/rules/u3p5.go
package rules

import (
	"context"

	"go.mongodb.org/mongo-driver/mongo"
)

// U3P5Rule: "Plant the superfruit seeds"
// Trigger: DialogueNodeEvent:10:194
// Logic: Attempt-based pos/neg scoring
//   - posCount = count of DialogueNodeEvent:73:163
//   - negCount = count of NEG_KEYS (73:164, 73:168, 73:171)
//   - sumScore = posCount * 1.0 - negCount * 0.5
// Green: sumScore >= 3
// Yellow: sumScore < 3
type U3P5Rule struct {
	BaseRule
}

// NewU3P5Rule creates a new U3P5 rule.
func NewU3P5Rule() *U3P5Rule {
	return &U3P5Rule{
		BaseRule: NewBaseRule(3, 5, "v1", []string{"DialogueNodeEvent:10:194"}),
	}
}

const u3p5Threshold = 3.0

// Evaluate calculates pos/neg score.
func (r *U3P5Rule) Evaluate(ctx context.Context, db *mongo.Database, game, playerID string) (Result, error) {
	helper := NewLogDataHelper(db, game)

	// Get the attempt window for this trigger
	triggerKey := "DialogueNodeEvent:10:194"
	window, err := helper.GetAttemptWindow(ctx, playerID, triggerKey)
	if err != nil {
		return Result{}, err
	}
	if window == nil {
		return Yellow("NO_TRIGGER", map[string]any{
			"reason": "No trigger event found",
		}), nil
	}

	// Count positive events: DialogueNodeEvent:73:163
	posKey := "DialogueNodeEvent:73:163"
	posCount, err := helper.CountEventInIDWindow(ctx, playerID, posKey, window)
	if err != nil {
		return Result{}, err
	}

	// Count negative events: 73:164, 73:168, 73:171
	negKeys := []string{
		"DialogueNodeEvent:73:164",
		"DialogueNodeEvent:73:168",
		"DialogueNodeEvent:73:171",
	}

	negCount, err := helper.CountEventsInWindow(ctx, playerID, negKeys, window)
	if err != nil {
		return Result{}, err
	}

	// Calculate score: posCount * 1.0 - negCount * 0.5
	score := float64(posCount)*1.0 - float64(negCount)*0.5

	if score < u3p5Threshold {
		return Yellow("LOW_SCORE", map[string]any{
			"score":     score,
			"posCount":  posCount,
			"negCount":  negCount,
			"threshold": u3p5Threshold,
		}), nil
	}

	return Green(), nil
}
