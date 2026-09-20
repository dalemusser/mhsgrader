package rules

import (
	"context"

	"go.mongodb.org/mongo-driver/mongo"
)

// U2P1Rule — Escape the Ruin: match the six topographic maps to their
// elevation profiles on your own within four attempts.
//
// Spec: mhsgrading/grading-logic/mhs-unit2-point1-grading.md (2026-09).
// Window: previous questFinishEvent:21 (exclusive) → this one (inclusive).
// Green iff the solved-on-their-own node (68:29) is present and no yellow
// node (4th-attempt-or-later feedback, forced assist) fired in the window.
// Reason SOLVED_WITH_ASSIST: an assist node fired (forced 68:28/68:31 or the
// accepted offer 68:24/68:32 followed by DANI placing the pieces 68:26/68:34);
// attempt_number = negative-feedback nodes in the window.
// Reason EXCESS_ATTEMPTS: solved on their own with no assist but 4+ negative
// nodes (success on attempt 5 or later); attempt_number = negatives + 1.
type U2P1Rule struct{ BaseRule }

func NewU2P1Rule() *U2P1Rule {
	return &U2P1Rule{NewBaseRule(2, 1, "v3",
		[]string{"DialogueNodeEvent:18:1"},
		[]string{"questFinishEvent:21"},
		WithWindow(WindowPrevTrigger),
	)}
}

func (r *U2P1Rule) Evaluate(ctx context.Context, db *mongo.Database, game, userID string, ec EvalContext) (Result, error) {
	helper := NewLogDataHelper(db, game)
	w := ec.Window

	const successKey = "DialogueNodeEvent:68:29" // solved-on-their-own completion

	yellowNodes := []string{
		"DialogueNodeEvent:68:22",
		"DialogueNodeEvent:68:23",
		"DialogueNodeEvent:68:27",
		"DialogueNodeEvent:68:28",
		"DialogueNodeEvent:68:31",
	}
	assistKeys := []string{
		"DialogueNodeEvent:68:24", // accepted offer after 4th attempt ("Sure. I'm stuck")
		"DialogueNodeEvent:68:26", // DANI places the pieces (accepted after 4th attempt)
		"DialogueNodeEvent:68:28", // forced assist, 5th attempt, >3 wrong
		"DialogueNodeEvent:68:31", // forced assist, 6th attempt, any wrong
		"DialogueNodeEvent:68:32", // accepted offer after 5th attempt ("Sure. I'm stuck")
		"DialogueNodeEvent:68:34", // DANI places the pieces (accepted after 5th attempt)
	}
	negativeKeys := []string{
		"DialogueNodeEvent:68:4",  // 1st attempt, 2-3 wrong
		"DialogueNodeEvent:68:5",  // 1st attempt, >3 wrong
		"DialogueNodeEvent:68:6",  // 2nd attempt, 2-3 wrong
		"DialogueNodeEvent:68:7",  // 2nd attempt, >3 wrong
		"DialogueNodeEvent:68:17", // 3rd attempt, 2-3 wrong
		"DialogueNodeEvent:68:18", // 3rd attempt, >3 wrong
		"DialogueNodeEvent:68:22", // 4th attempt, 2-3 wrong
		"DialogueNodeEvent:68:23", // 4th attempt, >3 wrong (assist offered)
		"DialogueNodeEvent:68:27", // 5th attempt, 2-3 wrong (assist offered)
		"DialogueNodeEvent:68:28", // 5th attempt, >3 wrong (DANI assists)
		"DialogueNodeEvent:68:31", // 6th attempt, any wrong (DANI assists)
	}

	hasSuccess, err := helper.HasEventInWindow(ctx, userID, successKey, w)
	if err != nil {
		return Result{}, err
	}
	yellowCount, err := helper.CountEventsInWindow(ctx, userID, yellowNodes, w)
	if err != nil {
		return Result{}, err
	}
	assisted, err := helper.HasAnyEventInWindow(ctx, userID, assistKeys, w)
	if err != nil {
		return Result{}, err
	}
	negCount, err := helper.CountEventsInWindow(ctx, userID, negativeKeys, w)
	if err != nil {
		return Result{}, err
	}

	metrics := map[string]any{
		"mistakeCount": negCount,
		"yellowCount":  yellowCount,
		"hasSuccess":   hasSuccess,
		"assisted":     assisted,
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
	if hasSuccess && !assisted && negCount >= 4 {
		reasons = append(reasons, Reason{
			Code:      "EXCESS_ATTEMPTS",
			Variables: map[string]any{"attempt_number": negCount + 1},
		})
	}
	return FlaggedWith(metrics, reasons...), nil
}
