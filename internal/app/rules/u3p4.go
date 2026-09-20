package rules

import (
	"context"

	"go.mongodb.org/mongo-driver/mongo"
)

// U3P4Rule — Forsaken Facility: the student orders the puzzle pieces showing
// how materials dissolve into water.
//
// Spec: mhsgrading/grading-logic/mhs-unit3-point4-grading.md (2026-09).
// Window: latest questActiveEvent:18 before the trigger (exclusive) →
// DialogueNodeEvent:73:200 (inclusive). The scripts require both anchors with
// the end after the start; without a start anchor they are yellow with no code.
// Green iff the completion gate 78:24 is in the window AND the 8 colour keys
// (7 attempt-feedback nodes + DANI's assist 78:23) count ≤ 2.
// Reasons: SOLVED_WITH_ASSIST when 78:23 fired or the gate is absent
// (attempt_number = 7-key count); EXCESS_ATTEMPTS when the gate is present,
// no assist, and the 8-key count ≥ 3 (attempt_number = 8-key count + 1).
type U3P4Rule struct{ BaseRule }

func NewU3P4Rule() *U3P4Rule {
	return &U3P4Rule{NewBaseRule(3, 4, "v3",
		[]string{"questActiveEvent:18"},
		[]string{"DialogueNodeEvent:73:200"},
	)}
}

func (r *U3P4Rule) Evaluate(ctx context.Context, db *mongo.Database, game, userID string, ec EvalContext) (Result, error) {
	helper := NewLogDataHelper(db, game)
	w := ec.Window

	const (
		gateKey   = "DialogueNodeEvent:78:24" // completion marker (empty text)
		assistKey = "DialogueNodeEvent:78:23" // DANI orders the pieces
	)
	negativeKeys := []string{
		"DialogueNodeEvent:78:4",  // 1st attempt, 1-2 wrong
		"DialogueNodeEvent:78:3",  // 1st attempt, 3-4 wrong
		"DialogueNodeEvent:78:7",  // 2nd attempt, any wrong (microscope hint)
		"DialogueNodeEvent:78:9",  // 3rd attempt, 1-2 wrong
		"DialogueNodeEvent:78:10", // 3rd attempt, 3-4 wrong
		"DialogueNodeEvent:78:12", // 4th attempt, 1-2 wrong
		"DialogueNodeEvent:78:18", // 4th attempt, 3-4 wrong (assist offered)
	}
	colorTargetKeys := append(append([]string(nil), negativeKeys...), assistKey)

	// The scripts need a valid (start, end] window: no questActiveEvent:18
	// before the trigger means yellow with no reason code.
	if ec.StartEventID.IsZero() {
		return FlaggedWith(map[string]any{
			"mistakeCount": int64(0),
			"validWindow":  false,
		}), nil
	}

	hasGate, err := helper.HasEventInWindow(ctx, userID, gateKey, w)
	if err != nil {
		return Result{}, err
	}
	assisted, err := helper.HasEventInWindow(ctx, userID, assistKey, w)
	if err != nil {
		return Result{}, err
	}
	negativeCount, err := helper.CountEventsInWindow(ctx, userID, negativeKeys, w)
	if err != nil {
		return Result{}, err
	}
	colorCount, err := helper.CountEventsInWindow(ctx, userID, colorTargetKeys, w)
	if err != nil {
		return Result{}, err
	}

	// score: 2 if count == 0, 1 if count <= 2, 0 if count >= 3
	var score int64
	switch {
	case colorCount == 0:
		score = 2
	case colorCount <= 2:
		score = 1
	}

	metrics := map[string]any{
		"mistakeCount":  colorCount,
		"hasGate":       hasGate,
		"assisted":      assisted,
		"negativeCount": negativeCount,
		"score":         score,
	}
	if hasGate && score != 0 {
		return PassedWithMetrics(metrics), nil
	}

	var reasons []Reason
	if assisted || !hasGate {
		reasons = append(reasons, Reason{
			Code:      "SOLVED_WITH_ASSIST",
			Variables: map[string]any{"attempt_number": negativeCount},
		})
	}
	if hasGate && !assisted && colorCount >= 3 {
		reasons = append(reasons, Reason{
			Code:      "EXCESS_ATTEMPTS",
			Variables: map[string]any{"attempt_number": colorCount + 1},
		})
	}
	return FlaggedWith(metrics, reasons...), nil
}
