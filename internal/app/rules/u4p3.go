package rules

import (
	"context"

	"go.mongodb.org/mongo-driver/mongo"
)

// U4P3Rule — Alien Well Floors 3 & 4: soil-machine canister changes per floor.
//
// Spec: mhsgrading/grading-logic/mhs-unit4-point3-grading.md (2026-09).
// Window: previous questActiveEvent:50 (exclusive) → this one (inclusive).
// Score: +1 if the third-floor machine ("1", floor "3") was changed exactly
// once; +2 if the fourth-floor machine (floor "4") was changed once, +1 if
// twice. Green iff score > 1.
// Reason SCORE_BELOW_THRESHOLD: floor3_attempts, floor4_attempts.
type U4P3Rule struct{ BaseRule }

func NewU4P3Rule() *U4P3Rule {
	return &U4P3Rule{NewBaseRule(4, 3, "v3",
		[]string{"questActiveEvent:48"},
		[]string{"questActiveEvent:50"},
		WithWindow(WindowPrevTrigger),
	)}
}

func (r *U4P3Rule) Evaluate(ctx context.Context, db *mongo.Database, game, userID string, ec EvalContext) (Result, error) {
	helper := NewLogDataHelper(db, game)
	w := ec.Window

	// data.floor / data.machine are strings; floor 5's machine "2" is excluded.
	floor3, err := helper.CountByEventTypeAndData(ctx, userID, "soilMachine",
		map[string]any{"machine": "1", "floor": "3"}, w)
	if err != nil {
		return Result{}, err
	}
	floor4, err := helper.CountByEventTypeAndData(ctx, userID, "soilMachine",
		map[string]any{"machine": "1", "floor": "4"}, w)
	if err != nil {
		return Result{}, err
	}

	score := int64(0)
	if floor3 == 1 {
		score += 1
	}
	if floor4 == 1 {
		score += 2
	} else if floor4 == 2 {
		score += 1
	}

	metrics := map[string]any{
		"floor3Attempts": floor3,
		"floor4Attempts": floor4,
		"score":          score,
		"mistakeCount":   floor3 + floor4,
	}
	if score > 1 {
		return PassedWithMetrics(metrics), nil
	}
	return FlaggedWith(metrics, Reason{
		Code: "SCORE_BELOW_THRESHOLD",
		Variables: map[string]any{
			"floor3_attempts": floor3,
			"floor4_attempts": floor4,
		},
	}), nil
}
