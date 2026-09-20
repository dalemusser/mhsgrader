package rules

import (
	"context"

	"go.mongodb.org/mongo-driver/mongo"
)

// U4P4Rule — Alien Well Floor 5 + You Know the Drill: fifth-floor soil
// machines plus the drill-depth choice.
//
// Spec: mhsgrading/grading-logic/mhs-unit4-point4-grading.md (2026-09).
// Window: latest questActiveEvent:50 before the trigger (exclusive) →
// questActiveEvent:36 (inclusive).
// Score: +1 if machine "1" has exactly one TopRow and one BottomRow change;
// +1 if machine "2" has exactly one change; +2 if a success choice (107:4,
// 107:5) with no colour-negative choice (107:2, 107:3, 107:6), +1 with one.
// Green iff score > 2.
// Reason SCORE_BELOW_THRESHOLD: machine_attempt_number (all fifth-floor
// canister changes), wrong_choice_number (wrong drill depths 107:2/3/4/6).
type U4P4Rule struct{ BaseRule }

func NewU4P4Rule() *U4P4Rule {
	return &U4P4Rule{NewBaseRule(4, 4, "v3",
		[]string{"questActiveEvent:50"},
		[]string{"questActiveEvent:36"},
	)}
}

func (r *U4P4Rule) Evaluate(ctx context.Context, db *mongo.Database, game, userID string, ec EvalContext) (Result, error) {
	helper := NewLogDataHelper(db, game)
	w := ec.Window

	// The colour scripts list 107:4 (middle — contaminated water) as a success
	// key; the spec flags that for review. Mirrored verbatim here.
	colorSuccessKeys := []string{"DialogueNodeEvent:107:4", "DialogueNodeEvent:107:5"}
	colorNegKeys := []string{"DialogueNodeEvent:107:2", "DialogueNodeEvent:107:3", "DialogueNodeEvent:107:6"}
	wrongDepthKeys := []string{
		"DialogueNodeEvent:107:2", // first floor — no water
		"DialogueNodeEvent:107:3", // second floor — no water
		"DialogueNodeEvent:107:4", // middle — contaminated water
		"DialogueNodeEvent:107:6", // all the way down — bedrock
	}

	machineCount := func(filter map[string]any) (int64, error) {
		return helper.CountByEventTypeAndData(ctx, userID, "soilMachine", filter, w)
	}
	m1Top, err := machineCount(map[string]any{"floor": "5", "machine": "1", "row": "TopRow"})
	if err != nil {
		return Result{}, err
	}
	m1Bottom, err := machineCount(map[string]any{"floor": "5", "machine": "1", "row": "BottomRow"})
	if err != nil {
		return Result{}, err
	}
	m2, err := machineCount(map[string]any{"floor": "5", "machine": "2"})
	if err != nil {
		return Result{}, err
	}

	successTotal, err := helper.CountEventsInWindow(ctx, userID, colorSuccessKeys, w)
	if err != nil {
		return Result{}, err
	}
	colorNegTotal, err := helper.CountEventsInWindow(ctx, userID, colorNegKeys, w)
	if err != nil {
		return Result{}, err
	}
	wrongDepths, err := helper.CountEventsInWindow(ctx, userID, wrongDepthKeys, w)
	if err != nil {
		return Result{}, err
	}

	score := int64(0)
	if m1Top == 1 && m1Bottom == 1 {
		score += 1
	}
	if m2 == 1 {
		score += 1
	}
	if successTotal > 0 && colorNegTotal == 0 {
		score += 2
	} else if successTotal > 0 && colorNegTotal == 1 {
		score += 1
	}

	metrics := map[string]any{
		"topRowAttempts":    m1Top,
		"bottomRowAttempts": m1Bottom,
		"machine2Attempts":  m2,
		"successCount":      successTotal,
		"negativeCount":     colorNegTotal,
		"wrongDepthCount":   wrongDepths,
		"score":             score,
		"mistakeCount":      wrongDepths,
	}
	if score > 2 {
		return PassedWithMetrics(metrics), nil
	}
	return FlaggedWith(metrics, Reason{
		Code: "SCORE_BELOW_THRESHOLD",
		Variables: map[string]any{
			"machine_attempt_number": m1Top + m1Bottom + m2,
			"wrong_choice_number":    wrongDepths,
		},
	}), nil
}
