package rules

import (
	"context"

	"go.mongodb.org/mongo-driver/mongo"
)

// U3P5Rule — Plant the Superfruit Seeds: the student plants four seeds in
// plots that receive the super-nutrient.
//
// Spec: mhsgrading/grading-logic/mhs-unit3-point5-grading.md (2026-09).
// Window: previous DialogueNodeEvent:10:194 (exclusive) → this one (inclusive).
// score = posCount·1.0 − negCount·0.5; green iff score ≥ 2.5 (the production
// script; the rule table's ≥ 3 is flagged for reconciliation in the doc).
// Reason EXCESS_WRONG_PLANTINGS: wrong_planting_number = wrong-spot feedback
// count (73:164 first wrong, 73:168 intermediate, 73:171 fourth wrong).
// EA U3.C5 (max 4): the same score, floored at 0 (the EA document's own
// formula: correct − ½ × incorrect).
type U3P5Rule struct{ BaseRule }

func NewU3P5Rule() *U3P5Rule {
	return &U3P5Rule{NewBaseRule(3, 5, "v3",
		[]string{"DialogueNodeEvent:73:200"},
		[]string{"DialogueNodeEvent:10:194"},
		WithWindow(WindowPrevTrigger),
	)}
}

func (r *U3P5Rule) Evaluate(ctx context.Context, db *mongo.Database, game, userID string, ec EvalContext) (Result, error) {
	helper := NewLogDataHelper(db, game)
	w := ec.Window

	const posKey = "DialogueNodeEvent:73:163"
	negKeys := []string{
		"DialogueNodeEvent:73:164", // 1st wrong spot
		"DialogueNodeEvent:73:168", // intermediate wrong spot (repeats)
		"DialogueNodeEvent:73:171", // 4th wrong spot, activity terminates
	}

	posCount, err := helper.CountEventInIDWindow(ctx, userID, posKey, w)
	if err != nil {
		return Result{}, err
	}
	negCount, err := helper.CountEventsInWindow(ctx, userID, negKeys, w)
	if err != nil {
		return Result{}, err
	}

	score := float64(posCount)*1.0 - float64(negCount)*0.5

	metrics := map[string]any{
		"mistakeCount":          negCount,
		"posCount":              posCount,
		"negCount":              negCount,
		"score":                 score,
		"wrong_planting_number": negCount,
	}
	ea := eaOne(EAU3C5, max(score, 0), 4)
	if score >= 2.5 {
		return PassedWithMetrics(metrics).WithEA(ea), nil
	}
	return FlaggedWith(metrics, Reason{
		Code:      "EXCESS_WRONG_PLANTINGS",
		Variables: map[string]any{"wrong_planting_number": negCount},
	}).WithEA(ea), nil
}
