package rules

import (
	"context"

	"go.mongodb.org/mongo-driver/mongo"
)

// U2P2Rule — Foraged Forging: at most one adaptive navigation reminder while
// searching for Captain Toppo.
//
// Spec: mhsgrading/grading-logic/mhs-unit2-point2-grading.md (2026-09).
// Window: latest questFinishEvent:21 before the trigger → DialogueNodeEvent:20:26,
// fenced by _id and by client timestamp. The script cannot bound the attempt
// without a start anchor and a timestamp on both anchors; that state is
// yellow with no code (the reason script returns triggered: false there).
// Green iff reminder count <= 1.
// Reason EXCESS_NAV_REMINDERS: count > 1; triggering_number = count.
type U2P2Rule struct{ BaseRule }

func NewU2P2Rule() *U2P2Rule {
	return &U2P2Rule{NewBaseRule(2, 2, "v3",
		[]string{"questFinishEvent:21"},
		[]string{"DialogueNodeEvent:20:26"},
		WithTimestampFence(),
	)}
}

func (r *U2P2Rule) Evaluate(ctx context.Context, db *mongo.Database, game, userID string, ec EvalContext) (Result, error) {
	helper := NewLogDataHelper(db, game)
	w := ec.Window

	targetKeys := []string{
		"DialogueNodeEvent:28:179", // clue reminder: near original location, rock formations
		"DialogueNodeEvent:59:179",
		"DialogueNodeEvent:28:182", // map-usage reminder: open the map with M
		"DialogueNodeEvent:59:182",
		"DialogueNodeEvent:28:183", // clue reminder: hill at ~90 feet elevation
		"DialogueNodeEvent:59:183",
	}

	count, err := helper.CountEventsInWindow(ctx, userID, targetKeys, w)
	if err != nil {
		return Result{}, err
	}

	metrics := map[string]any{"mistakeCount": count}

	if ec.StartEventID.IsZero() || w.TSStart == nil || w.TSEnd == nil {
		metrics["windowInvalid"] = "start anchor or client timestamp missing; yellow by rule"
		return FlaggedWith(metrics), nil
	}
	if count <= 1 {
		return PassedWithMetrics(metrics), nil
	}
	return FlaggedWith(metrics, Reason{
		Code:      "EXCESS_NAV_REMINDERS",
		Variables: map[string]any{"triggering_number": count},
	}), nil
}
