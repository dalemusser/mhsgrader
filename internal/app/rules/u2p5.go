package rules

import (
	"context"

	"go.mongodb.org/mongo-driver/mongo"
)

// U2P5Rule — Classified Information: classify the highlighted passages of a
// scientific argument as claim, reasoning or evidence.
//
// Spec: mhsgrading/grading-logic/mhs-unit2-point5-grading.md (2026-09).
// Window: previous DialogueNodeEvent:23:42 (exclusive) → this one (inclusive).
// score = posCount − negCount/3; green iff score >= 4.
// Reason EXCESS_MISCLASSIFICATIONS: score < 4; wrong_number = negatives,
// claim_wrong / reasoning_wrong / evidence_wrong = negatives split by what
// the misclassified passage actually was.
// EA U2.C5 (max 6, team decision D2): the raw score as computed here
// (+1 per correct placement, −⅓ per incorrect), floored at 0.
type U2P5Rule struct{ BaseRule }

func NewU2P5Rule() *U2P5Rule {
	return &U2P5Rule{NewBaseRule(2, 5, "v3",
		[]string{"DialogueNodeEvent:23:17"},
		[]string{"DialogueNodeEvent:23:42"},
		WithWindow(WindowPrevTrigger),
	)}
}

func (r *U2P5Rule) Evaluate(ctx context.Context, db *mongo.Database, game, userID string, ec EvalContext) (Result, error) {
	helper := NewLogDataHelper(db, game)
	w := ec.Window

	posKeys := []string{
		// claim correct
		"DialogueNodeEvent:26:140", "DialogueNodeEvent:26:146", "DialogueNodeEvent:26:165",
		"DialogueNodeEvent:26:168", "DialogueNodeEvent:26:172", "DialogueNodeEvent:26:175",
		"DialogueNodeEvent:26:178", "DialogueNodeEvent:26:181", "DialogueNodeEvent:26:184",
		// reasoning correct
		"DialogueNodeEvent:26:142", "DialogueNodeEvent:26:147", "DialogueNodeEvent:26:166",
		"DialogueNodeEvent:26:169", "DialogueNodeEvent:26:173", "DialogueNodeEvent:26:176",
		"DialogueNodeEvent:26:179", "DialogueNodeEvent:26:182", "DialogueNodeEvent:26:185",
		// evidence correct
		"DialogueNodeEvent:26:143", "DialogueNodeEvent:26:148", "DialogueNodeEvent:26:167",
		"DialogueNodeEvent:26:170", "DialogueNodeEvent:26:174", "DialogueNodeEvent:26:177",
		"DialogueNodeEvent:26:180", "DialogueNodeEvent:26:183", "DialogueNodeEvent:26:186",
	}
	claimNegKeys := []string{ // passage was a claim
		"DialogueNodeEvent:26:137", "DialogueNodeEvent:26:187", "DialogueNodeEvent:26:191",
		"DialogueNodeEvent:26:194", "DialogueNodeEvent:26:197", "DialogueNodeEvent:26:200",
		"DialogueNodeEvent:26:203", "DialogueNodeEvent:26:206", "DialogueNodeEvent:26:209",
	}
	reasoningNegKeys := []string{ // passage was reasoning
		"DialogueNodeEvent:26:144", "DialogueNodeEvent:26:188", "DialogueNodeEvent:26:192",
		"DialogueNodeEvent:26:195", "DialogueNodeEvent:26:198", "DialogueNodeEvent:26:201",
		"DialogueNodeEvent:26:204", "DialogueNodeEvent:26:207", "DialogueNodeEvent:26:210",
	}
	evidenceNegKeys := []string{ // passage was evidence
		"DialogueNodeEvent:26:145", "DialogueNodeEvent:26:189", "DialogueNodeEvent:26:193",
		"DialogueNodeEvent:26:196", "DialogueNodeEvent:26:199", "DialogueNodeEvent:26:202",
		"DialogueNodeEvent:26:205", "DialogueNodeEvent:26:208", "DialogueNodeEvent:26:211",
	}
	negKeys := make([]string, 0, len(claimNegKeys)+len(reasoningNegKeys)+len(evidenceNegKeys))
	negKeys = append(negKeys, claimNegKeys...)
	negKeys = append(negKeys, reasoningNegKeys...)
	negKeys = append(negKeys, evidenceNegKeys...)

	posCount, err := helper.CountEventsInWindow(ctx, userID, posKeys, w)
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

	score := float64(posCount) - (float64(negCount) / 3.0)

	metrics := map[string]any{
		"mistakeCount":   negCount,
		"posCount":       posCount,
		"score":          score,
		"claimWrong":     claimWrong,
		"reasoningWrong": reasoningWrong,
		"evidenceWrong":  evidenceWrong,
	}
	ea := eaOne(EAU2C5, max(score, 0), 6)
	if score >= 4 {
		return PassedWithMetrics(metrics).WithEA(ea), nil
	}
	return FlaggedWith(metrics, Reason{
		Code: "EXCESS_MISCLASSIFICATIONS",
		Variables: map[string]any{
			"wrong_number":    negCount,
			"claim_wrong":     claimWrong,
			"reasoning_wrong": reasoningWrong,
			"evidence_wrong":  evidenceWrong,
		},
	}).WithEA(ea), nil
}
