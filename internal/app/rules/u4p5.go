package rules

import (
	"context"

	"go.mongodb.org/mongo-driver/mongo"
)

// U4P5Rule — Saving Cadet Anderson: build the flooding argument with at most
// 2 incorrect submissions.
//
// Spec: mhsgrading/grading-logic/mhs-unit4-point5-grading.md (2026-09).
// Window: previous questActiveEvent:41 (exclusive) → this one (inclusive).
// Green iff a success node (90:50 first try, 90:57 after revisions) is in the
// window and fewer than 3 negative feedback nodes are.
// Reason EXCESS_ATTEMPTS: attempt_number (incorrect submissions, + 1 when
// success is present), claim_wrong_number, reasoning_wrong_number,
// evidence_wrong_number.
type U4P5Rule struct{ BaseRule }

func NewU4P5Rule() *U4P5Rule {
	return &U4P5Rule{NewBaseRule(4, 5, "v3",
		[]string{"questActiveEvent:36"},
		[]string{"questActiveEvent:41"},
		WithWindow(WindowPrevTrigger),
	)}
}

func (r *U4P5Rule) Evaluate(ctx context.Context, db *mongo.Database, game, userID string, ec EvalContext) (Result, error) {
	helper := NewLogDataHelper(db, game)
	w := ec.Window

	posKeys := []string{"DialogueNodeEvent:90:50", "DialogueNodeEvent:90:57"}
	claimNegKeys := []string{
		"DialogueNodeEvent:90:37", "DialogueNodeEvent:90:55",
	}
	reasoningNegKeys := []string{
		"DialogueNodeEvent:90:25", "DialogueNodeEvent:90:56",
		"DialogueNodeEvent:90:52", "DialogueNodeEvent:90:60", // bedrock misconception
		"DialogueNodeEvent:90:54", "DialogueNodeEvent:90:61", // infiltrate-upward misconception
	}
	evidenceNegKeys := []string{
		"DialogueNodeEvent:90:39", "DialogueNodeEvent:90:58", // missing evidence
		"DialogueNodeEvent:90:45", "DialogueNodeEvent:90:59", // unsupportive evidence
		"DialogueNodeEvent:90:47", // incomplete argument
	}
	// NEG_KEYS = claim + reasoning + evidence (the 13 negative feedback nodes).
	negKeys := make([]string, 0, len(claimNegKeys)+len(reasoningNegKeys)+len(evidenceNegKeys))
	negKeys = append(negKeys, claimNegKeys...)
	negKeys = append(negKeys, reasoningNegKeys...)
	negKeys = append(negKeys, evidenceNegKeys...)

	hasSuccess, err := helper.HasAnyEventInWindow(ctx, userID, posKeys, w)
	if err != nil {
		return Result{}, err
	}
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
		"hasSuccess":     hasSuccess,
		"negativeCount":  negCount,
		"claimWrong":     claimWrong,
		"reasoningWrong": reasoningWrong,
		"evidenceWrong":  evidenceWrong,
	}
	if hasSuccess && negCount < 3 {
		return PassedWithMetrics(metrics), nil
	}

	attemptNumber := negCount
	if hasSuccess {
		attemptNumber++
	}
	return FlaggedWith(metrics, Reason{
		Code: "EXCESS_ATTEMPTS",
		Variables: map[string]any{
			"attempt_number":         attemptNumber,
			"claim_wrong_number":     claimWrong,
			"reasoning_wrong_number": reasoningWrong,
			"evidence_wrong_number":  evidenceWrong,
		},
	}), nil
}
