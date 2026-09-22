package rules

import (
	"context"

	"go.mongodb.org/mongo-driver/mongo"
)

// U3P1Rule — Establishing a Foothold (supply run): the student floats Tera's
// three crates down the river that flows past her camp.
//
// Spec: mhsgrading/grading-logic/mhs-unit3-point1-grading.md (2026-09).
// Window: previous DialogueNodeEvent:11:22 (exclusive) → this one (inclusive).
// Green iff more than one correct-river confirmation (10:30) in the window.
// Reason EXCESS_WRONG_RIVERS: wrong_river_number = wrong-river feedback nodes
// (10:31 mid-task, 10:32 last crate) counted directly in the window.
// EA U3.C1 (max 3): one point per correct crate placement (10:30), at most 3.
type U3P1Rule struct{ BaseRule }

func NewU3P1Rule() *U3P1Rule {
	return &U3P1Rule{NewBaseRule(3, 1, "v3",
		[]string{"DialogueNodeEvent:10:1"},
		[]string{"DialogueNodeEvent:11:22"},
		WithWindow(WindowPrevTrigger),
	)}
}

func (r *U3P1Rule) Evaluate(ctx context.Context, db *mongo.Database, game, userID string, ec EvalContext) (Result, error) {
	helper := NewLogDataHelper(db, game)
	w := ec.Window

	const correctKey = "DialogueNodeEvent:10:30" // "you picked the right river"
	wrongRiverKeys := []string{
		"DialogueNodeEvent:10:31", // wrong river, crate lost (crates 1-2)
		"DialogueNodeEvent:10:32", // wrong river, last crate
	}

	correctCount, err := helper.CountEventInIDWindow(ctx, userID, correctKey, w)
	if err != nil {
		return Result{}, err
	}
	wrongCount, err := helper.CountEventsInWindow(ctx, userID, wrongRiverKeys, w)
	if err != nil {
		return Result{}, err
	}

	metrics := map[string]any{
		"mistakeCount":       wrongCount,
		"correctCount":       correctCount,
		"wrong_river_number": wrongCount,
	}
	ea := eaOne(EAU3C1, float64(min(correctCount, 3)), 3)
	if correctCount > 1 {
		return PassedWithMetrics(metrics).WithEA(ea), nil
	}
	return FlaggedWith(metrics, Reason{
		Code:      "EXCESS_WRONG_RIVERS",
		Variables: map[string]any{"wrong_river_number": wrongCount},
	}).WithEA(ea), nil
}
