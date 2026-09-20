package rules

import (
	"context"

	"go.mongodb.org/mongo-driver/mongo"
)

// U5P3Rule — What Happened Here?: Dr. Toppo's wrong-answer feedback in the
// argument about why the collected water disappeared.
//
// Spec: mhsgrading/grading-logic/mhs-unit5-point3-grading.md (2026-09).
// Window: latest DialogueNodeEvent:96:1 (exclusive) → latest questFinishEvent:44 (inclusive).
// Each flagged submission fires exactly one of the 39 conversation-108
// feedback nodes below (generic/specific pairs both counted). Yellow iff the
// count is ≥ 4; no success node is required.
// Reason EXCESS_ATTEMPTS: wrong_argument_number = total, split into
// claim_wrong_number / reasoning_wrong_number / evidence_wrong_number.
type U5P3Rule struct{ BaseRule }

func NewU5P3Rule() *U5P3Rule {
	return &U5P3Rule{NewBaseRule(5, 3, "v3",
		[]string{"DialogueNodeEvent:96:1"},
		[]string{"questFinishEvent:44"},
		WithWindow(WindowStartBeforeEnd),
	)}
}

func (r *U5P3Rule) Evaluate(ctx context.Context, db *mongo.Database, game, userID string, ec EvalContext) (Result, error) {
	helper := NewLogDataHelper(db, game)
	w := ec.Window

	claimNegKeys := []string{ // restating Aryn's claim
		"DialogueNodeEvent:108:32", "DialogueNodeEvent:108:33", "DialogueNodeEvent:108:37",
		"DialogueNodeEvent:108:41", "DialogueNodeEvent:108:70", "DialogueNodeEvent:108:72",
		"DialogueNodeEvent:108:73", "DialogueNodeEvent:108:74", "DialogueNodeEvent:108:75",
		"DialogueNodeEvent:108:76", "DialogueNodeEvent:108:78", "DialogueNodeEvent:108:79",
	}
	reasoningNegKeys := []string{
		"DialogueNodeEvent:108:39", "DialogueNodeEvent:108:53", // too many reasoning statements
		"DialogueNodeEvent:108:54", "DialogueNodeEvent:108:55", // redundant reasoning
		"DialogueNodeEvent:108:90", "DialogueNodeEvent:108:91", // too many reasoning statements
		"DialogueNodeEvent:108:59", "DialogueNodeEvent:108:84", // "water being filtered" misconception
		"DialogueNodeEvent:108:85",
		"DialogueNodeEvent:108:61", "DialogueNodeEvent:108:86", // "transformed into salt" misconception
		"DialogueNodeEvent:108:60", "DialogueNodeEvent:108:62", // reasoning doesn't match the argument
		"DialogueNodeEvent:108:87",
		"DialogueNodeEvent:108:88", "DialogueNodeEvent:108:89", // reasoning doesn't connect all evidence (C and D)
		"DialogueNodeEvent:108:65", "DialogueNodeEvent:108:66", // reasoning doesn't connect all evidence (C or D alone)
	}
	evidenceNegKeys := []string{
		"DialogueNodeEvent:108:25", "DialogueNodeEvent:108:80", // salt amount doesn't explain the water
		"DialogueNodeEvent:108:82", "DialogueNodeEvent:108:83", // evidence doesn't support the claim (A alone)
		"DialogueNodeEvent:108:68", "DialogueNodeEvent:108:69", // evidence doesn't support the claim (pair other than C+D)
		"DialogueNodeEvent:108:63", "DialogueNodeEvent:108:64", // one piece of evidence missing (C or D alone, reasoning 3)
		"DialogueNodeEvent:108:47", // incomplete argument
	}

	var negKeys []string
	negKeys = append(negKeys, claimNegKeys...)
	negKeys = append(negKeys, reasoningNegKeys...)
	negKeys = append(negKeys, evidenceNegKeys...)

	negCount, err := helper.CountEventsInWindow(ctx, userID, negKeys, w)
	if err != nil {
		return Result{}, err
	}
	claimWrong, err := helper.CountEventsInWindow(ctx, userID, claimNegKeys, w)
	if err != nil {
		return Result{}, err
	}
	reasoningWrong, err := helper.CountEventsInWindow(ctx, userID, reasoningNegKeys, w)
	if err != nil {
		return Result{}, err
	}
	evidenceWrong, err := helper.CountEventsInWindow(ctx, userID, evidenceNegKeys, w)
	if err != nil {
		return Result{}, err
	}

	metrics := map[string]any{
		"mistakeCount":   negCount,
		"claimWrong":     claimWrong,
		"reasoningWrong": reasoningWrong,
		"evidenceWrong":  evidenceWrong,
	}
	if negCount < 4 {
		return PassedWithMetrics(metrics), nil
	}
	return FlaggedWith(metrics, Reason{
		Code: "EXCESS_ATTEMPTS",
		Variables: map[string]any{
			"wrong_argument_number":  negCount,
			"claim_wrong_number":     claimWrong,
			"reasoning_wrong_number": reasoningWrong,
			"evidence_wrong_number":  evidenceWrong,
		},
	}), nil
}
