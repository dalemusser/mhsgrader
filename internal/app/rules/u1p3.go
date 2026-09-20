package rules

import (
	"context"

	"go.mongodb.org/mongo-driver/mongo"
)

// U1P3Rule — Defend the Expedition: the first argument submitted must be the
// correct one.
//
// Spec: mhsgrading/grading-logic/mhs-unit1-point3-grading.md (2026-09).
// Window: previous questActiveEvent:34 (exclusive) → this one (inclusive).
// Green iff no wrong-claim feedback (70:25) in the window.
// Reason WRONG_ARG_SELECTED: attempt_number = wrong submissions + the accepted one.
type U1P3Rule struct{ BaseRule }

func NewU1P3Rule() *U1P3Rule {
	return &U1P3Rule{NewBaseRule(1, 3, "v3",
		[]string{"DialogueNodeEvent:30:98"},
		[]string{"questActiveEvent:34"},
		WithWindow(WindowPrevTrigger),
	)}
}

func (r *U1P3Rule) Evaluate(ctx context.Context, db *mongo.Database, game, userID string, ec EvalContext) (Result, error) {
	helper := NewLogDataHelper(db, game)
	w := ec.Window

	const (
		wrongKey   = "DialogueNodeEvent:70:25" // Toppo: that claim is not the one to argue
		successKey = "DialogueNodeEvent:70:7"  // argument accepted
	)

	wrong, err := helper.CountEventInIDWindow(ctx, userID, wrongKey, w)
	if err != nil {
		return Result{}, err
	}
	attempts, err := helper.CountEventsInWindow(ctx, userID, []string{wrongKey, successKey}, w)
	if err != nil {
		return Result{}, err
	}

	metrics := map[string]any{
		"mistakeCount":   wrong,
		"attempt_number": attempts,
	}
	if wrong == 0 {
		return PassedWithMetrics(metrics), nil
	}
	return FlaggedWith(metrics, Reason{
		Code:      "WRONG_ARG_SELECTED",
		Variables: map[string]any{"attempt_number": attempts},
	}), nil
}
