package rules

import (
	"context"

	"go.mongodb.org/mongo-driver/mongo"
)

// U5P1Rule — If I Had a Nickel (floors 1 & 2): the evaporation glyph puzzle
// must be solved independently within 4 attempts.
//
// Spec: mhsgrading/grading-logic/mhs-unit5-point1-grading.md (2026-09).
// Window: latest questActiveEvent:43 (exclusive) → latest questFinishEvent:43 (inclusive).
// Green iff the success node 100:44 fired and none of the colour negatives
// (100:38, 100:39, 100:43 — the 4th-attempt-and-later feedback) did.
// Reason SOLVED_WITH_ASSIST: an assist-path marker fired; attempt_number = incorrect arrangements.
// Reason EXCESS_ATTEMPTS: no assist marker but a colour negative fired;
// attempt_number = incorrect arrangements + 1 (the final correct submission).
type U5P1Rule struct{ BaseRule }

func NewU5P1Rule() *U5P1Rule {
	return &U5P1Rule{NewBaseRule(5, 1, "v3",
		[]string{"questActiveEvent:43"},
		[]string{"questFinishEvent:43"},
		WithWindow(WindowStartBeforeEnd),
	)}
}

func (r *U5P1Rule) Evaluate(ctx context.Context, db *mongo.Database, game, userID string, ec EvalContext) (Result, error) {
	helper := NewLogDataHelper(db, game)
	w := ec.Window

	// "Well done, TK" — fires on BOTH the independent and the DANI-assisted
	// paths, so it is not an independence signal.
	const successKey = "DialogueNodeEvent:100:44"

	colorNegKeys := []string{
		"DialogueNodeEvent:100:38", // 4th attempt, close
		"DialogueNodeEvent:100:39", // 4th attempt, far (assist offered)
		"DialogueNodeEvent:100:43", // 5th attempt, forced assist
	}
	assistKeys := []string{
		"DialogueNodeEvent:100:40", // "Sure, I'm stuck" — offer accepted
		"DialogueNodeEvent:100:41", // accepted-assist execution
		"DialogueNodeEvent:100:43", // forced assist (5th attempt)
		"DialogueNodeEvent:100:46", // DANI-helped completion
	}
	negativeKeys := []string{
		"DialogueNodeEvent:100:33", // 1st attempt, close
		"DialogueNodeEvent:100:34", // 1st attempt, far
		"DialogueNodeEvent:100:35", // 2nd attempt (evaporation-rate hint)
		"DialogueNodeEvent:100:36", // 3rd attempt, close
		"DialogueNodeEvent:100:37", // 3rd attempt, far (temperature hint)
		"DialogueNodeEvent:100:38", // 4th attempt, close ("very close")
		"DialogueNodeEvent:100:39", // 4th attempt, far (assist offered)
	}

	hasSuccess, err := helper.HasEventInWindow(ctx, userID, successKey, w)
	if err != nil {
		return Result{}, err
	}
	colorNegCount, err := helper.CountEventsInWindow(ctx, userID, colorNegKeys, w)
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
		"mistakeCount":  negCount, // incorrect arrangements
		"colorNegCount": colorNegCount,
		"hasSuccess":    hasSuccess,
		"assisted":      assisted,
	}
	if hasSuccess && colorNegCount == 0 {
		return PassedWithMetrics(metrics), nil
	}

	var reasons []Reason
	if assisted {
		reasons = append(reasons, Reason{
			Code:      "SOLVED_WITH_ASSIST",
			Variables: map[string]any{"attempt_number": negCount},
		})
	} else if colorNegCount > 0 {
		reasons = append(reasons, Reason{
			Code:      "EXCESS_ATTEMPTS",
			Variables: map[string]any{"attempt_number": negCount + 1},
		})
	}
	return FlaggedWith(metrics, reasons...), nil
}
