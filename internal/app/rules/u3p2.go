package rules

import (
	"context"

	"go.mongodb.org/mongo-driver/mongo"
)

// U3P2Rule — Pollution Solution: the student traces the pollution source
// with drone-dropped sensors.
//
// Spec: mhsgrading/grading-logic/mhs-unit3-point2-grading.md (2026-09).
// Window: previous DialogueNodeEvent:11:34 (exclusive) → this one (inclusive).
// The start key is questActiveEvent:17 (the doc's questFinishEvent:17 fires
// after the end trigger and would open phantom attempts).
// score = 5 − cappedPenalty(c27) − cappedPenalty(c29 + c230); green iff ≥ 3.
// Reason EXCESS_SENSOR_REMINDERS: downstream_reminder_number = 11:27,
// redundant_reminder_number = 11:29 + 11:230.
type U3P2Rule struct{ BaseRule }

func NewU3P2Rule() *U3P2Rule {
	return &U3P2Rule{NewBaseRule(3, 2, "v3",
		[]string{"questActiveEvent:17"},
		[]string{"DialogueNodeEvent:11:34"},
		WithWindow(WindowPrevTrigger),
	)}
}

// cappedPenalty returns penalty: 0 if cnt <= 1, 1 if cnt <= 3, 2 if cnt >= 4.
func cappedPenalty(cnt int64) int64 {
	if cnt <= 1 {
		return 0
	}
	if cnt <= 3 {
		return 1
	}
	return 2
}

func (r *U3P2Rule) Evaluate(ctx context.Context, db *mongo.Database, game, userID string, ec EvalContext) (Result, error) {
	helper := NewLogDataHelper(db, game)
	w := ec.Window

	const (
		downstreamKey = "DialogueNodeEvent:11:27"  // test further upstream
		cleanKey      = "DialogueNodeEvent:11:29"  // no need to check upstream of a clean sensor
		topKey        = "DialogueNodeEvent:11:230" // top of branch reached, proceed downstream
	)

	c27, err := helper.CountEventInIDWindow(ctx, userID, downstreamKey, w)
	if err != nil {
		return Result{}, err
	}
	c29, err := helper.CountEventInIDWindow(ctx, userID, cleanKey, w)
	if err != nil {
		return Result{}, err
	}
	c230, err := helper.CountEventInIDWindow(ctx, userID, topKey, w)
	if err != nil {
		return Result{}, err
	}
	redundantCount := c29 + c230

	score := 5 - cappedPenalty(c27) - cappedPenalty(redundantCount)

	metrics := map[string]any{
		"mistakeCount":               c27 + redundantCount,
		"c27":                        c27,
		"c29":                        c29,
		"c230":                       c230,
		"score":                      score,
		"downstream_reminder_number": c27,
		"redundant_reminder_number":  redundantCount,
	}
	if score >= 3 {
		return PassedWithMetrics(metrics), nil
	}
	return FlaggedWith(metrics, Reason{
		Code: "EXCESS_SENSOR_REMINDERS",
		Variables: map[string]any{
			"downstream_reminder_number": c27,
			"redundant_reminder_number":  redundantCount,
		},
	}), nil
}
