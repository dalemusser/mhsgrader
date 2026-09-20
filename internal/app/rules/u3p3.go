package rules

import (
	"context"

	"go.mongodb.org/mongo-driver/mongo"
)

// U3P3Rule — Pollution Argument: the student argues where the pollutant
// enters the river.
//
// Spec: mhsgrading/grading-logic/mhs-unit3-point3-grading.md (2026-09).
// Window: previous questFinishEvent:18 (exclusive) → this one (inclusive).
// Colour: sumCount over the 18 colour keys (which include success node 84:36
// on purpose, so the count equals the number of attempts, and the inert gate
// 84:38); base score 3/2/1/0 for ≤3/4/5/≥6; +1 when the Pollution Site Data
// backing-info panel was opened; green iff total ≥ 3.
// Reason EXCESS_ATTEMPTS: honest counts — wrong_argument_number excludes
// 84:36/38 and the three component counts split it by argument state.
type U3P3Rule struct{ BaseRule }

func NewU3P3Rule() *U3P3Rule {
	return &U3P3Rule{NewBaseRule(3, 3, "v3",
		[]string{"DialogueNodeEvent:11:34"},
		[]string{"questFinishEvent:18"},
		WithWindow(WindowPrevTrigger),
	)}
}

// u3p3BaseScore: 3 if count <= 3, 2 if == 4, 1 if == 5, 0 if >= 6.
func u3p3BaseScore(count int64) int64 {
	if count <= 3 {
		return 3
	}
	if count == 4 {
		return 2
	}
	if count == 5 {
		return 1
	}
	return 0
}

func (r *U3P3Rule) Evaluate(ctx context.Context, db *mongo.Database, game, userID string, ec EvalContext) (Result, error) {
	helper := NewLogDataHelper(db, game)
	w := ec.Window

	claimNegKeys := []string{
		"DialogueNodeEvent:84:39", "DialogueNodeEvent:84:45", // claim II + evidence A: only the claim is wrong
		"DialogueNodeEvent:84:25", "DialogueNodeEvent:84:46", // claim II + evidence B, reasoning 1/2/3/5: claim problem named
		"DialogueNodeEvent:84:40", // claim II + evidence B, reasoning 4: same gate
	}
	reasoningNegKeys := []string{ // claim I + evidence A: only the reasoning is wrong
		"DialogueNodeEvent:84:32", "DialogueNodeEvent:84:41", // reasoning 1: does not explain the pollution's location
		"DialogueNodeEvent:84:33", "DialogueNodeEvent:84:42", // reasoning 2: does not explain how water behaves
		"DialogueNodeEvent:84:34", "DialogueNodeEvent:84:43", // reasoning 3: "water must flow north to south"
		"DialogueNodeEvent:84:35", "DialogueNodeEvent:84:44", // reasoning 4: does not match the evidence
	}
	evidenceStructNegKeys := []string{
		"DialogueNodeEvent:84:37", // claim I + evidence B: evidence doesn't match the argument
		"DialogueNodeEvent:84:20", // multiple evidence pieces used
		"DialogueNodeEvent:84:47", // incomplete argument
	}

	var wrongKeys []string
	wrongKeys = append(wrongKeys, claimNegKeys...)
	wrongKeys = append(wrongKeys, reasoningNegKeys...)
	wrongKeys = append(wrongKeys, evidenceStructNegKeys...)

	// Colour-rule key list, kept verbatim so the colour matches the cell (incl. 36/38).
	colorTargetKeys := append(append([]string(nil), wrongKeys...),
		"DialogueNodeEvent:84:36", "DialogueNodeEvent:84:38")

	const backingInfoTool = "BackingInfoPanel - Pollution Site Data"

	sumCount, err := helper.CountEventsInWindow(ctx, userID, colorTargetKeys, w) // mirrors the colour script
	if err != nil {
		return Result{}, err
	}
	wrongCount, err := helper.CountEventsInWindow(ctx, userID, wrongKeys, w) // honest wrong-submission count
	if err != nil {
		return Result{}, err
	}
	claimCount, err := helper.CountEventsInWindow(ctx, userID, claimNegKeys, w)
	if err != nil {
		return Result{}, err
	}
	reasoningCount, err := helper.CountEventsInWindow(ctx, userID, reasoningNegKeys, w)
	if err != nil {
		return Result{}, err
	}
	evidenceCount, err := helper.CountEventsInWindow(ctx, userID, evidenceStructNegKeys, w)
	if err != nil {
		return Result{}, err
	}
	hasBonus, err := helper.HasEventTypeAndData(ctx, userID, "argumentationToolEvent",
		map[string]any{"toolName": backingInfoTool}, w)
	if err != nil {
		return Result{}, err
	}

	baseScore := u3p3BaseScore(sumCount)
	totalScore := baseScore
	if hasBonus {
		totalScore++
	}
	backingInfoPhrase := "did not open"
	if hasBonus {
		backingInfoPhrase = "opened"
	}

	metrics := map[string]any{
		"mistakeCount":           wrongCount,
		"sumCount":               sumCount,
		"baseScore":              baseScore,
		"usedBackingInfo":        hasBonus,
		"totalScore":             totalScore,
		"wrong_argument_number":  wrongCount,
		"claim_wrong_number":     claimCount,
		"reasoning_wrong_number": reasoningCount,
		"evidence_wrong_number":  evidenceCount,
	}
	if totalScore >= 3 {
		return PassedWithMetrics(metrics), nil
	}
	return FlaggedWith(metrics, Reason{
		Code: "EXCESS_ATTEMPTS",
		Variables: map[string]any{
			"wrong_argument_number":  wrongCount,
			"claim_wrong_number":     claimCount,
			"reasoning_wrong_number": reasoningCount,
			"evidence_wrong_number":  evidenceCount,
			"backing_info_phrase":    backingInfoPhrase,
		},
	}), nil
}
