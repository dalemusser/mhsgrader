package rules

import (
	"context"

	"go.mongodb.org/mongo-driver/mongo"
)

// U2P4Rule — Investigate the Temple: order the watershed terrain pieces on
// your own within five attempts.
//
// Spec: mhsgrading/grading-logic/mhs-unit2-point4-grading.md (2026-09).
// Window: previous DialogueNodeEvent:23:17 (exclusive) → this one (inclusive).
// Green iff the solved-on-their-own node (74:21) is present and no bad
// feedback node (74:16, 74:17, 74:20, 74:22) fired in the window.
// Reason SOLVED_WITH_ASSIST: an assist marker fired (74:18 accepted offer,
// 74:20/74:25 DANI orders the pieces, 74:22 helped completion);
// attempt_number = feedback nodes in the window.
// Reason EXCESS_ATTEMPTS: solved on their own, no assist marker, and the 5th
// submission was wrong (74:16 or 74:17); attempt_number = negatives + 1.
type U2P4Rule struct{ BaseRule }

func NewU2P4Rule() *U2P4Rule {
	return &U2P4Rule{NewBaseRule(2, 4, "v3",
		[]string{"DialogueNodeEvent:22:18"},
		[]string{"DialogueNodeEvent:23:17"},
		WithWindow(WindowPrevTrigger),
	)}
}

func (r *U2P4Rule) Evaluate(ctx context.Context, db *mongo.Database, game, userID string, ec EvalContext) (Result, error) {
	helper := NewLogDataHelper(db, game)
	w := ec.Window

	const successKey = "DialogueNodeEvent:74:21" // solved-on-their-own completion

	badKeys := []string{
		"DialogueNodeEvent:74:16",
		"DialogueNodeEvent:74:17",
		"DialogueNodeEvent:74:20",
		"DialogueNodeEvent:74:22",
	}
	assistKeys := []string{
		"DialogueNodeEvent:74:18", // "Sure. I'm stuck" — accepted assist offer
		"DialogueNodeEvent:74:20", // DANI orders the pieces
		"DialogueNodeEvent:74:22", // DANI-helped completion
		"DialogueNodeEvent:74:25", // DANI orders the pieces (video-link variant)
	}
	fifthAttemptKeys := []string{
		"DialogueNodeEvent:74:16", // 5th attempt, 2-3 wrong
		"DialogueNodeEvent:74:17", // 5th attempt, >3 wrong (assist offered)
	}
	negativeKeys := []string{
		"DialogueNodeEvent:74:4",  // 1st attempt, any wrong
		"DialogueNodeEvent:74:5",  // 2nd attempt, 2-3 wrong
		"DialogueNodeEvent:74:6",  // 2nd attempt, >3 wrong
		"DialogueNodeEvent:74:9",  // 3rd attempt, 2-3 wrong
		"DialogueNodeEvent:74:10", // 3rd attempt, >3 wrong (video offered)
		"DialogueNodeEvent:74:15", // 4th attempt, any wrong
		"DialogueNodeEvent:74:16", // 5th attempt, 2-3 wrong
		"DialogueNodeEvent:74:17", // 5th attempt, >3 wrong (assist offered)
	}

	hasSuccess, err := helper.HasEventInWindow(ctx, userID, successKey, w)
	if err != nil {
		return Result{}, err
	}
	badCount, err := helper.CountEventsInWindow(ctx, userID, badKeys, w)
	if err != nil {
		return Result{}, err
	}
	assisted, err := helper.HasAnyEventInWindow(ctx, userID, assistKeys, w)
	if err != nil {
		return Result{}, err
	}
	fifthWrong, err := helper.HasAnyEventInWindow(ctx, userID, fifthAttemptKeys, w)
	if err != nil {
		return Result{}, err
	}
	negCount, err := helper.CountEventsInWindow(ctx, userID, negativeKeys, w)
	if err != nil {
		return Result{}, err
	}

	metrics := map[string]any{
		"mistakeCount": negCount,
		"badCount":     badCount,
		"hasSuccess":   hasSuccess,
		"assisted":     assisted,
		"fifthWrong":   fifthWrong,
	}
	if hasSuccess && badCount == 0 {
		return PassedWithMetrics(metrics), nil
	}

	var reasons []Reason
	if assisted {
		reasons = append(reasons, Reason{
			Code:      "SOLVED_WITH_ASSIST",
			Variables: map[string]any{"attempt_number": negCount},
		})
	}
	if hasSuccess && !assisted && fifthWrong {
		reasons = append(reasons, Reason{
			Code:      "EXCESS_ATTEMPTS",
			Variables: map[string]any{"attempt_number": negCount + 1},
		})
	}
	return FlaggedWith(metrics, reasons...), nil
}
