package rules

import (
	"context"

	"go.mongodb.org/mongo-driver/mongo"
)

// U5P2Rule — If I Had a Nickel (floors 3 & 4): condenser/evaporator
// interactions per floor, scored.
//
// Spec: mhsgrading/grading-logic/mhs-unit5-point2-grading.md (2026-09).
// Window: latest questFinishEvent:43 (exclusive) → latest DialogueNodeEvent:96:1 (inclusive).
// floor3 ≤ 6 → +2, 7–10 → +1; floor4 ≤ 5 → +2, 6–9 → +1. Green iff score ≥ 3.
// Only machineType Condenser/Evaporator count (the DualChamber_* types are a
// gap flagged in the markdown, kept mirror-exact with the colour script).
// Reason SCORE_BELOW_THRESHOLD: floor3_attempts, floor4_attempts.
type U5P2Rule struct{ BaseRule }

func NewU5P2Rule() *U5P2Rule {
	return &U5P2Rule{NewBaseRule(5, 2, "v3",
		[]string{"questFinishEvent:43"},
		[]string{"DialogueNodeEvent:96:1"},
		WithWindow(WindowStartBeforeEnd),
	)}
}

func (r *U5P2Rule) Evaluate(ctx context.Context, db *mongo.Database, game, userID string, ec EvalContext) (Result, error) {
	helper := NewLogDataHelper(db, game)
	w := ec.Window

	validTypes := []string{"Condenser", "Evaporator"}

	// floorCount mirrors the script's countDocuments with
	// "data.machineType": { $in: VALID_TYPES } as one count per type, summed.
	floorCount := func(floor string) (int64, error) {
		var total int64
		for _, machineType := range validTypes {
			n, err := helper.CountByEventTypeAndData(ctx, userID, "WaterChamberEvent",
				map[string]any{"floor": floor, "machineType": machineType}, w)
			if err != nil {
				return 0, err
			}
			total += n
		}
		return total, nil
	}

	floor3, err := floorCount("3")
	if err != nil {
		return Result{}, err
	}
	floor4, err := floorCount("4")
	if err != nil {
		return Result{}, err
	}

	score := 0
	if floor3 <= 6 {
		score += 2
	} else if floor3 < 11 {
		score += 1
	}
	if floor4 <= 5 {
		score += 2
	} else if floor4 < 10 {
		score += 1
	}

	metrics := map[string]any{
		"floor3Attempts": floor3,
		"floor4Attempts": floor4,
		"score":          score,
		"mistakeCount":   floor3 + floor4,
	}
	if score >= 3 {
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
