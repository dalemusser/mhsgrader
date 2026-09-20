package rules

import (
	"context"

	"go.mongodb.org/mongo-driver/mongo"
)

// U2P6Rule — Which Watershed? Part I: pick water flow rate as the strongest
// evidence for the larger watershed on the first try.
//
// Spec: mhsgrading/grading-logic/mhs-unit2-point6-grading.md (2026-09).
// Window: latest DialogueNodeEvent:23:42 before the trigger (exclusive) →
// DialogueNodeEvent:20:46 (inclusive). The script has no window without a
// start anchor; that state is yellow with no code.
// Green iff the pass node (20:43) is present and neither wrong-choice node
// (20:44 waterfall height, 20:45 salinity) fired in the window.
// Reason WRONG_EVIDENCE_SELECTED: a wrong-choice node fired; wrong_choice
// names the option(s) chosen.
type U2P6Rule struct{ BaseRule }

func NewU2P6Rule() *U2P6Rule {
	return &U2P6Rule{NewBaseRule(2, 6, "v3",
		[]string{"DialogueNodeEvent:23:42"},
		[]string{"DialogueNodeEvent:20:46"},
	)}
}

func (r *U2P6Rule) Evaluate(ctx context.Context, db *mongo.Database, game, userID string, ec EvalContext) (Result, error) {
	helper := NewLogDataHelper(db, game)
	w := ec.Window

	const (
		passKey     = "DialogueNodeEvent:20:43" // chose water flow rate
		heightKey   = "DialogueNodeEvent:20:44" // chose waterfall height
		salinityKey = "DialogueNodeEvent:20:45" // chose salinity
	)

	hasPass, err := helper.HasEventInWindow(ctx, userID, passKey, w)
	if err != nil {
		return Result{}, err
	}
	heightCount, err := helper.CountEventInIDWindow(ctx, userID, heightKey, w)
	if err != nil {
		return Result{}, err
	}
	salinityCount, err := helper.CountEventInIDWindow(ctx, userID, salinityKey, w)
	if err != nil {
		return Result{}, err
	}
	hasHeight, hasSalinity := heightCount > 0, salinityCount > 0

	metrics := map[string]any{
		"mistakeCount":  heightCount + salinityCount,
		"hasPass":       hasPass,
		"heightCount":   heightCount,
		"salinityCount": salinityCount,
	}

	if ec.StartEventID.IsZero() {
		metrics["windowInvalid"] = "no DialogueNodeEvent:23:42 before the trigger; yellow by rule"
		return FlaggedWith(metrics), nil
	}
	if hasPass && !hasHeight && !hasSalinity {
		return PassedWithMetrics(metrics), nil
	}

	wrongChoice := ""
	switch {
	case hasHeight && hasSalinity:
		wrongChoice = "waterfall height and salinity"
	case hasHeight:
		wrongChoice = "waterfall height"
	case hasSalinity:
		wrongChoice = "salinity"
	}

	var reasons []Reason
	if wrongChoice != "" {
		reasons = append(reasons, Reason{
			Code:      "WRONG_EVIDENCE_SELECTED",
			Variables: map[string]any{"wrong_choice": wrongChoice},
		})
	}
	return FlaggedWith(metrics, reasons...), nil
}
