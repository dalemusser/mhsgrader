package rules

import (
	"context"

	"go.mongodb.org/mongo-driver/mongo"
)

// U5P2Rule — If I Had a Nickel (Floors 3-4): WaterChamberEvent attempt scoring.
// Floor 3: +2 if <= 6 attempts, +1 if < 11.
// Floor 4: +2 if <= 5 attempts, +1 if < 10.
// Green if score >= 3; Flagged if < 3.
type U5P2Rule struct{ BaseRule }

func NewU5P2Rule() *U5P2Rule {
	return &U5P2Rule{NewBaseRule(5, 2, "v2",
		[]string{"questFinishEvent:43"},
		[]string{"DialogueNodeEvent:96:1"},
	)}
}

func (r *U5P2Rule) Evaluate(ctx context.Context, db *mongo.Database, game, playerID string) (Result, error) {
	helper := NewLogDataHelper(db, game)

	window, err := helper.GetAttemptWindow(ctx, playerID, "DialogueNodeEvent:96:1")
	if err != nil {
		return Result{}, err
	}
	if window == nil {
		return Flagged("NO_TRIGGER", nil), nil
	}

	// Count WaterChamberEvent for floor 3 (Condenser or Evaporator)
	cFloor3Cond, err := helper.CountByEventTypeAndData(ctx, playerID, "WaterChamberEvent",
		map[string]string{"floor": "3", "machineType": "Condenser"}, window)
	if err != nil {
		return Result{}, err
	}
	cFloor3Evap, err := helper.CountByEventTypeAndData(ctx, playerID, "WaterChamberEvent",
		map[string]string{"floor": "3", "machineType": "Evaporator"}, window)
	if err != nil {
		return Result{}, err
	}
	floor3 := cFloor3Cond + cFloor3Evap

	// Count WaterChamberEvent for floor 4 (Condenser or Evaporator)
	cFloor4Cond, err := helper.CountByEventTypeAndData(ctx, playerID, "WaterChamberEvent",
		map[string]string{"floor": "4", "machineType": "Condenser"}, window)
	if err != nil {
		return Result{}, err
	}
	cFloor4Evap, err := helper.CountByEventTypeAndData(ctx, playerID, "WaterChamberEvent",
		map[string]string{"floor": "4", "machineType": "Evaporator"}, window)
	if err != nil {
		return Result{}, err
	}
	floor4 := cFloor4Cond + cFloor4Evap

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

	if score >= 3 {
		return Passed(), nil
	}
	return Flagged("SCORE_BELOW_THRESHOLD", map[string]any{
		"score": score, "floor3_attempts": floor3, "floor4_attempts": floor4,
	}), nil
}
