package rules

import (
	"context"

	"go.mongodb.org/mongo-driver/mongo"
)

// U2P3Rule — Getting the Band Back Together Part II: fewer than six adaptive
// navigation reminders across the searches for Tera and Aryn.
//
// Spec: mhsgrading/grading-logic/mhs-unit2-point3-grading.md (2026-09).
// Window: latest DialogueNodeEvent:20:33 before the trigger → DialogueNodeEvent:22:18
// (the script's START_KEY; the doc header's Trigger(Start) 20:26 is not used).
// The colour script fences by _id and by client timestamp and is yellow when
// the start anchor or either timestamp is missing; the reason script uses the
// same window by _id only and guards only on the start anchor.
// Green iff the fenced reminder count < 6.
// Reason EXCESS_NAV_REMINDERS: _id-window count >= 6; triggering_number = that
// count, tera_count = reminders from the start to the first 21:1 (Tera's
// greeting; the window end when absent), aryn_count = reminders from the
// first 18:231 (Aryn waypoint prompt) to the end (0 when absent).
// EA U2.C3 (max 3, team decision D1): the Tera search and the Aryn search
// scored separately by their own help-dialog sets (the EA document's rows
// 2.3 and 2.4: once or less 1½, two or three 1, four ½, more 0) and summed;
// only when the window has a start anchor.
type U2P3Rule struct{ BaseRule }

func NewU2P3Rule() *U2P3Rule {
	return &U2P3Rule{NewBaseRule(2, 3, "v3",
		[]string{"DialogueNodeEvent:20:33"},
		[]string{"DialogueNodeEvent:22:18"},
		WithTimestampFence(),
	)}
}

func (r *U2P3Rule) Evaluate(ctx context.Context, db *mongo.Database, game, userID string, ec EvalContext) (Result, error) {
	helper := NewLogDataHelper(db, game)
	w := ec.Window

	const (
		teraFoundKey = "DialogueNodeEvent:21:1"   // Tera's greeting: the Tera search is over
		arynStartKey = "DialogueNodeEvent:18:231" // Aryn waypoint prompt: the Aryn search begins
	)

	teraKeys := []string{ // the Tera search's help dialogs (EA row 2.3)
		"DialogueNodeEvent:18:225", "DialogueNodeEvent:28:185", "DialogueNodeEvent:59:185",
		"DialogueNodeEvent:28:184", "DialogueNodeEvent:28:191", "DialogueNodeEvent:59:184", "DialogueNodeEvent:59:191",
		"DialogueNodeEvent:18:226", "DialogueNodeEvent:18:227", "DialogueNodeEvent:28:186", "DialogueNodeEvent:59:186",
		"DialogueNodeEvent:18:228", "DialogueNodeEvent:28:187", "DialogueNodeEvent:59:187",
		"DialogueNodeEvent:18:229", "DialogueNodeEvent:28:188", "DialogueNodeEvent:59:188",
		"DialogueNodeEvent:18:230", "DialogueNodeEvent:28:180", "DialogueNodeEvent:59:180",
	}
	arynKeys := []string{ // the Aryn search's help dialogs (EA row 2.4)
		"DialogueNodeEvent:18:233", "DialogueNodeEvent:28:192", "DialogueNodeEvent:59:192",
		"DialogueNodeEvent:18:234", "DialogueNodeEvent:28:193", "DialogueNodeEvent:59:193",
		"DialogueNodeEvent:18:235", "DialogueNodeEvent:28:194", "DialogueNodeEvent:59:194",
		"DialogueNodeEvent:18:236", "DialogueNodeEvent:18:237", "DialogueNodeEvent:28:190", "DialogueNodeEvent:59:190",
	}
	targetKeys := append(append([]string{}, teraKeys...), arynKeys...)

	// Colour: the timestamp-fenced count.
	fencedCount, err := helper.CountEventsInWindow(ctx, userID, targetKeys, w)
	if err != nil {
		return Result{}, err
	}

	// Reason: the same window by _id only, split into the two searches. The
	// phase markers are not target keys, so the exclusive lower bound of Sub
	// is count-neutral.
	idWindow := w.Sub(w.StartID, w.EndID)
	total, err := helper.CountEventsInWindow(ctx, userID, targetKeys, idWindow)
	if err != nil {
		return Result{}, err
	}
	teraEnd, err := helper.EarliestEventInWindow(ctx, userID, []string{teraFoundKey}, idWindow)
	if err != nil {
		return Result{}, err
	}
	arynStart, err := helper.EarliestEventInWindow(ctx, userID, []string{arynStartKey}, idWindow)
	if err != nil {
		return Result{}, err
	}
	teraEndID := w.EndID
	if teraEnd != nil {
		teraEndID = teraEnd.ID
	}
	teraCount, err := helper.CountEventsInWindow(ctx, userID, targetKeys, w.Sub(w.StartID, teraEndID))
	if err != nil {
		return Result{}, err
	}
	var arynCount int64
	if arynStart != nil {
		arynCount, err = helper.CountEventsInWindow(ctx, userID, targetKeys, w.Sub(arynStart.ID, w.EndID))
		if err != nil {
			return Result{}, err
		}
	}

	metrics := map[string]any{
		"mistakeCount": fencedCount,
		"totalCount":   total,
		"teraCount":    teraCount,
		"arynCount":    arynCount,
	}

	hasStart := !ec.StartEventID.IsZero()

	// EA: each search by its own key set over the _id window.
	var ea map[string]EAScore
	if hasStart {
		teraHelp, err := helper.CountEventsInWindow(ctx, userID, teraKeys, idWindow)
		if err != nil {
			return Result{}, err
		}
		arynHelp, err := helper.CountEventsInWindow(ctx, userID, arynKeys, idWindow)
		if err != nil {
			return Result{}, err
		}
		metrics["eaTeraHelpCount"] = teraHelp
		metrics["eaArynHelpCount"] = arynHelp
		ea = eaOne(EAU2C3, EAHelpBand15(teraHelp)+EAHelpBand15(arynHelp), 3)
	}
	windowValid := hasStart && w.TSStart != nil && w.TSEnd != nil
	if !windowValid {
		metrics["windowInvalid"] = "start anchor or client timestamp missing; yellow by rule"
	}
	if windowValid && fencedCount < 6 {
		return PassedWithMetrics(metrics).WithEA(ea), nil
	}

	var reasons []Reason
	if hasStart && total >= 6 {
		reasons = append(reasons, Reason{
			Code: "EXCESS_NAV_REMINDERS",
			Variables: map[string]any{
				"triggering_number": total,
				"tera_count":        teraCount,
				"aryn_count":        arynCount,
			},
		})
	}
	return FlaggedWith(metrics, reasons...).WithEA(ea), nil
}
