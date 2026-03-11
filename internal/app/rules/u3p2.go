// internal/app/rules/u3p2.go
package rules

import (
	"context"

	"go.mongodb.org/mongo-driver/mongo"
)

// U3P2Rule: "Pollution solution"
// Trigger: DialogueNodeEvent:11:34
// Logic: Attempt-based capped penalty scoring
//   - c27 = count of 11:27, c29 = count of 11:29, c230 = count of 11:230
//   - cSum = c29 + c230
//   - cappedPenalty(cnt): 0 if <=1, 1 if <=3, 2 if >=4
//   - score = 5 - cappedPenalty(c27) - cappedPenalty(cSum)
// Green: score >= 3
// Yellow: score < 3
type U3P2Rule struct {
	BaseRule
}

// NewU3P2Rule creates a new U3P2 rule.
func NewU3P2Rule() *U3P2Rule {
	return &U3P2Rule{
		BaseRule: NewBaseRule(3, 2, "v1", []string{"DialogueNodeEvent:11:34"}),
	}
}

const u3p2Threshold = 3

// cappedPenalty returns penalty based on count:
// 0 if cnt <= 1, 1 if cnt <= 3, 2 if cnt >= 4
func cappedPenalty(cnt int64) int {
	if cnt <= 1 {
		return 0
	}
	if cnt <= 3 {
		return 1
	}
	return 2
}

// Evaluate calculates score using capped penalty scoring.
func (r *U3P2Rule) Evaluate(ctx context.Context, db *mongo.Database, game, playerID string) (Result, error) {
	helper := NewLogDataHelper(db, game)

	// Get the attempt window for this trigger
	triggerKey := "DialogueNodeEvent:11:34"
	window, err := helper.GetAttemptWindow(ctx, playerID, triggerKey)
	if err != nil {
		return Result{}, err
	}
	if window == nil {
		return Yellow("NO_TRIGGER", map[string]any{
			"reason": "No trigger event found",
		}), nil
	}

	// Count events
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

	// Calculate score
	cSum := c29 + c230
	score := 5 - cappedPenalty(c27) - cappedPenalty(cSum)

	if score < u3p2Threshold {
		return Yellow("LOW_SCORE", map[string]any{
			"score":     score,
			"c27":       c27,
			"c29":       c29,
			"c230":      c230,
			"threshold": u3p2Threshold,
		}), nil
	}

	return Green(), nil
}
