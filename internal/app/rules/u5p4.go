package rules

import (
	"context"
	"strconv"
	"strings"

	"go.mongodb.org/mongo-driver/mongo"
)

// U5P4Rule — Water Problems Require Water Solutions: the solar desalinator
// must collect the maximum water with no failed runs.
//
// Spec: mhsgrading/grading-logic/mhs-unit5-point4-grading.md (2026-09).
// Window: latest questFinishEvent:45 (inclusive), previous questFinishEvent:44
// before it (exclusive). Each run produces exactly one conversation-106
// outcome node. Green iff the success node 106:35 fired AND no failure
// outcome fired (zero tolerance).
// Reason WRONG_SETTINGS_SELECTED: wrong_run_number = failed runs;
// failure_phrase names the observed failure mode(s).
// EA U5.C4 (max 1½, team decision D6): ½ each for Tilted Out, Uncovered and
// Cold, read from the settings the FIRST outcome node of the attempt names
// (EASolarStillOutcomes); only when the window has a start anchor.
type U5P4Rule struct{ BaseRule }

func NewU5P4Rule() *U5P4Rule {
	return &U5P4Rule{NewBaseRule(5, 4, "v3",
		[]string{"questFinishEvent:44"},
		[]string{"questFinishEvent:45"},
		WithWindow(WindowStartBeforeEnd),
	)}
}

func (r *U5P4Rule) Evaluate(ctx context.Context, db *mongo.Database, game, userID string, ec EvalContext) (Result, error) {
	helper := NewLogDataHelper(db, game)
	w := ec.Window

	// "You set the solar desalinator to its best settings..."
	const successKey = "DialogueNodeEvent:106:35"

	sunlightKeys := []string{ // no water: sunlight blocked, no evaporation
		"DialogueNodeEvent:106:4", "DialogueNodeEvent:106:25", "DialogueNodeEvent:106:26",
		"DialogueNodeEvent:106:27", "DialogueNodeEvent:106:28", "DialogueNodeEvent:106:29",
	}
	glassKeys := []string{ // no water: glass too hot, no condensation
		"DialogueNodeEvent:106:30", "DialogueNodeEvent:106:31", "DialogueNodeEvent:106:32",
	}
	roofKeys := []string{ // small amount: roof angle didn't collect the water
		"DialogueNodeEvent:106:33", "DialogueNodeEvent:106:34",
	}

	hasSuccess, err := helper.HasEventInWindow(ctx, userID, successKey, w)
	if err != nil {
		return Result{}, err
	}
	sunlightCount, err := helper.CountEventsInWindow(ctx, userID, sunlightKeys, w)
	if err != nil {
		return Result{}, err
	}
	glassCount, err := helper.CountEventsInWindow(ctx, userID, glassKeys, w)
	if err != nil {
		return Result{}, err
	}
	roofCount, err := helper.CountEventsInWindow(ctx, userID, roofKeys, w)
	if err != nil {
		return Result{}, err
	}
	negCount := sunlightCount + glassCount + roofCount

	metrics := map[string]any{
		"mistakeCount":  negCount,
		"hasSuccess":    hasSuccess,
		"sunlightCount": sunlightCount,
		"glassCount":    glassCount,
		"roofCount":     roofCount,
	}

	// EA: the first submission's settings.
	var ea map[string]EAScore
	if !ec.StartEventID.IsZero() {
		first, err := helper.EarliestEventInWindow(ctx, userID, EASolarStillKeys(), w)
		if err != nil {
			return Result{}, err
		}
		if first != nil {
			if o, known := EASolarStillOutcomes[first.EventKey]; known {
				metrics["eaFirstOutcome"] = first.EventKey
				metrics["eaFirstRoof"] = o.Roof
				metrics["eaFirstUncovered"] = o.Uncovered
				metrics["eaFirstCold"] = o.Cold
				ea = eaOne(EAU5C4, o.Score(), 1.5)
			}
		}
	}

	if hasSuccess && negCount == 0 {
		return PassedWithMetrics(metrics).WithEA(ea), nil
	}

	// Mirror the script's failure_phrase word for word.
	runs := func(n int64) string {
		if n > 1 {
			return " (" + strconv.FormatInt(n, 10) + " runs)"
		}
		return ""
	}
	var parts []string
	if sunlightCount > 0 {
		parts = append(parts, "the settings blocked sunlight, so the salt water could not heat up and evaporate"+runs(sunlightCount))
	}
	if glassCount > 0 {
		parts = append(parts, "the glass surface was too hot for condensation to form"+runs(glassCount))
	}
	if roofCount > 0 {
		parts = append(parts, "the roof angle let most of the condensed water escape, collecting only a small amount"+runs(roofCount))
	}
	failurePhrase := "no successful desalinator run was recorded"
	if len(parts) > 0 {
		failurePhrase = strings.Join(parts, "; and ")
	}

	return FlaggedWith(metrics, Reason{
		Code: "WRONG_SETTINGS_SELECTED",
		Variables: map[string]any{
			"wrong_run_number": negCount,
			"failure_phrase":   failurePhrase,
		},
	}).WithEA(ea), nil
}
