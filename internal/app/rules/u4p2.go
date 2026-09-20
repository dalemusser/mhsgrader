package rules

import (
	"context"

	"go.mongodb.org/mongo-driver/mongo"
)

// U4P2Rule — Infiltration Glyph + Alien Well Floors 1 & 2: order the soils by
// infiltration rate independently within 3 attempts.
//
// Spec: mhsgrading/grading-logic/mhs-unit4-point2-grading.md (2026-09).
// Window: latest Unit-4 soil-key puzzle close before the trigger (exclusive;
// unbounded when none) → questActiveEvent:48 (inclusive).
// Green iff 88:11 is in the window and none of the yellow keys is.
// Reason SOLVED_WITH_ASSIST: DANI ordered the pieces (forced 102:23, or the
// accepted-offer path 102:20/102:21); attempt_number = incorrect arrangements.
// Reason EXCESS_ATTEMPTS: no assist, but a 3rd-or-later submission was wrong;
// attempt_number = incorrect arrangements + 1.
type U4P2Rule struct{ BaseRule }

func NewU4P2Rule() *U4P2Rule {
	return &U4P2Rule{NewBaseRule(4, 2, "v3",
		nil,
		[]string{"questActiveEvent:48"},
		WithStartMatchers(soilKeyClose),
	)}
}

func (r *U4P2Rule) Evaluate(ctx context.Context, db *mongo.Database, game, userID string, ec EvalContext) (Result, error) {
	helper := NewLogDataHelper(db, game)
	w := ec.Window

	const successKey = "DialogueNodeEvent:88:11" // post-puzzle explanation (fires on the assisted path too)

	// The colour rule's yellow keys: attempt-3-and-later feedback.
	yellowKeys := []string{
		"DialogueNodeEvent:102:9",  // 3rd attempt, 1-2 wrong
		"DialogueNodeEvent:102:10", // 3rd attempt, 3-4 wrong
		"DialogueNodeEvent:102:12", // 4th attempt, 1-2 wrong
		"DialogueNodeEvent:102:18", // 4th attempt, 3-4 wrong (assist offered)
		"DialogueNodeEvent:102:23", // 5th attempt, forced assist
	}
	// DANI completed the puzzle.
	assistKeys := []string{
		"DialogueNodeEvent:102:20", // accepted DANI's offer (after 102:18)
		"DialogueNodeEvent:102:21", // DANI orders the pieces (accepted path)
		"DialogueNodeEvent:102:23", // DANI orders the pieces (forced, 5th attempt)
	}
	// A wrong 3rd-or-later submission (the EXCESS_ATTEMPTS script's YELLOW_KEYS).
	lateWrongKeys := []string{
		"DialogueNodeEvent:102:9",
		"DialogueNodeEvent:102:10",
		"DialogueNodeEvent:102:12",
		"DialogueNodeEvent:102:18",
	}
	// Exactly one node per incorrect arrangement.
	negativeKeys := []string{
		"DialogueNodeEvent:102:4",  // 1st attempt, 1-2 wrong
		"DialogueNodeEvent:102:3",  // 1st attempt, 3-4 wrong
		"DialogueNodeEvent:102:7",  // 2nd attempt, any wrong (particle-size hint)
		"DialogueNodeEvent:102:9",  // 3rd attempt, 1-2 wrong
		"DialogueNodeEvent:102:10", // 3rd attempt, 3-4 wrong (rate-graph hint)
		"DialogueNodeEvent:102:12", // 4th attempt, 1-2 wrong
		"DialogueNodeEvent:102:18", // 4th attempt, 3-4 wrong (assist offered)
	}

	hasSuccess, err := helper.HasEventInWindow(ctx, userID, successKey, w)
	if err != nil {
		return Result{}, err
	}
	yellowCount, err := helper.CountEventsInWindow(ctx, userID, yellowKeys, w)
	if err != nil {
		return Result{}, err
	}
	assisted, err := helper.HasAnyEventInWindow(ctx, userID, assistKeys, w)
	if err != nil {
		return Result{}, err
	}
	hasLateWrong, err := helper.HasAnyEventInWindow(ctx, userID, lateWrongKeys, w)
	if err != nil {
		return Result{}, err
	}
	negCount, err := helper.CountEventsInWindow(ctx, userID, negativeKeys, w)
	if err != nil {
		return Result{}, err
	}

	metrics := map[string]any{
		"mistakeCount":  negCount,
		"hasSuccess":    hasSuccess,
		"yellowCount":   yellowCount,
		"assisted":      assisted,
		"negativeCount": negCount,
	}
	if hasSuccess && yellowCount == 0 {
		return PassedWithMetrics(metrics), nil
	}

	var reasons []Reason
	if assisted {
		reasons = append(reasons, Reason{
			Code:      "SOLVED_WITH_ASSIST",
			Variables: map[string]any{"attempt_number": negCount},
		})
	}
	if !assisted && hasLateWrong {
		reasons = append(reasons, Reason{
			Code:      "EXCESS_ATTEMPTS",
			Variables: map[string]any{"attempt_number": negCount + 1},
		})
	}
	return FlaggedWith(metrics, reasons...), nil
}
