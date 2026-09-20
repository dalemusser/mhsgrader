package rules

import (
	"context"

	"go.mongodb.org/mongo-driver/mongo"
)

// U2P7Rule — Which Watershed? Part II: build the argument about which
// watershed is larger within four submissions.
//
// Spec: mhsgrading/grading-logic/mhs-unit2-point7-grading.md (2026-09).
// Window: previous questFinishEvent:54 (exclusive) → this one (inclusive).
// Green iff the success node (27:7) is present and at most 3 incorrect
// submissions fired in the window.
// Reason EXCESS_ATTEMPTS: yellow by the colour rule; attempt_number =
// negatives + 1 when the success node is present (else negatives), and the
// negatives split by argument state: wrong_claim_number (claim I with the
// flow-rate evidence), both_wrong_number (claim I with irrelevant evidence),
// irrelevant_evidence_number (claim II with irrelevant evidence, or several
// pieces of evidence at once).
type U2P7Rule struct{ BaseRule }

func NewU2P7Rule() *U2P7Rule {
	return &U2P7Rule{NewBaseRule(2, 7, "v3",
		[]string{"DialogueNodeEvent:20:46"},
		[]string{"questFinishEvent:54"},
		WithWindow(WindowPrevTrigger),
	)}
}

func (r *U2P7Rule) Evaluate(ctx context.Context, db *mongo.Database, game, userID string, ec EvalContext) (Result, error) {
	helper := NewLogDataHelper(db, game)
	w := ec.Window

	const successKey = "DialogueNodeEvent:27:7" // "Well done! You have made the best argument possible."

	wrongClaimKeys := []string{ // claim I with the flow-rate evidence (A): only the claim is wrong
		"DialogueNodeEvent:27:11", // generic: evidence doesn't fit the claim (backing-info pointer)
		"DialogueNodeEvent:27:12", // specific: try a more appropriate claim
	}
	bothWrongKeys := []string{ // claim I with irrelevant evidence: claim AND evidence wrong
		"DialogueNodeEvent:27:13", "DialogueNodeEvent:27:14", // waterfall height (B)
		"DialogueNodeEvent:27:15", "DialogueNodeEvent:27:16", // salinity (C)
		"DialogueNodeEvent:27:17", "DialogueNodeEvent:27:18", // downstream river (D)
	}
	irrelevantEvidenceKeys := []string{ // claim II (correct) with evidence that does not indicate watershed size
		"DialogueNodeEvent:27:25", "DialogueNodeEvent:27:26", // waterfall height (B)
		"DialogueNodeEvent:27:27", "DialogueNodeEvent:27:28", // salinity (C)
		"DialogueNodeEvent:27:29", "DialogueNodeEvent:27:30", // downstream river (D)
		"DialogueNodeEvent:27:20", // several pieces of evidence at once (any claim)
	}
	negKeys := make([]string, 0, len(wrongClaimKeys)+len(bothWrongKeys)+len(irrelevantEvidenceKeys))
	negKeys = append(negKeys, wrongClaimKeys...)
	negKeys = append(negKeys, bothWrongKeys...)
	negKeys = append(negKeys, irrelevantEvidenceKeys...)

	hasSuccess, err := helper.HasEventInWindow(ctx, userID, successKey, w)
	if err != nil {
		return Result{}, err
	}
	negCount, err := helper.CountEventsInWindow(ctx, userID, negKeys, w)
	if err != nil {
		return Result{}, err
	}
	claimCount, err := helper.CountEventsInWindow(ctx, userID, wrongClaimKeys, w)
	if err != nil {
		return Result{}, err
	}
	bothCount, err := helper.CountEventsInWindow(ctx, userID, bothWrongKeys, w)
	if err != nil {
		return Result{}, err
	}
	evidenceCount, err := helper.CountEventsInWindow(ctx, userID, irrelevantEvidenceKeys, w)
	if err != nil {
		return Result{}, err
	}

	metrics := map[string]any{
		"mistakeCount":            negCount,
		"hasSuccess":              hasSuccess,
		"wrongClaimCount":         claimCount,
		"bothWrongCount":          bothCount,
		"irrelevantEvidenceCount": evidenceCount,
	}
	if hasSuccess && negCount <= 3 {
		return PassedWithMetrics(metrics), nil
	}

	attemptNumber := negCount
	if hasSuccess {
		attemptNumber = negCount + 1
	}
	return FlaggedWith(metrics, Reason{
		Code: "EXCESS_ATTEMPTS",
		Variables: map[string]any{
			"attempt_number":             attemptNumber,
			"wrong_claim_number":         claimCount,
			"both_wrong_number":          bothCount,
			"irrelevant_evidence_number": evidenceCount,
		},
	}), nil
}
