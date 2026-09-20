package rules

import (
	"context"
	"fmt"

	"github.com/dalemusser/mhsgrader/internal/app/store/logdata"
	"go.mongodb.org/mongo-driver/mongo"
)

// soilKeyClose is the close of the Unit-4 soil-key puzzle: the `Soil Key
// Puzzle` event with status "Finished" in a Unit 4 scene (the puzzle also
// fires in Units 2 and 3). These events carry no eventKey, so they are
// matched by eventType + data. It is U4P1's end trigger and U4P2's start
// anchor.
var soilKeyClose = logdata.TypeMatch("Soil Key Puzzle", map[string]any{
	"Soil Key Puzzle Status": "Finished",
	"Unit":                   UnitScene(4),
})

// U4P1Rule — Well What Have We Here?: the water-table question plus the
// soil-key puzzle timing.
//
// Spec: mhsgrading/grading-logic/mhs-unit4-point1-grading.md (2026-09).
// Window: latest DialogueNodeEvent:88:0 (exclusive) → the Unit-4 soil-key
// puzzle close (inclusive).
// Score: +0.5 if 88:5 (correct answer) is in the window; +1.0 if the puzzle
// took 0 < d <= 30 s, +0.5 if 30 < d <= 90 s, where d runs from the earliest
// Unit-4 "Started" soil-key event in the window to the close (serverTimestamp).
// Green iff score >= 1.
// Reason SCORE_BELOW_THRESHOLD: choice_phrase, duration_phrase.
type U4P1Rule struct{ BaseRule }

func NewU4P1Rule() *U4P1Rule {
	return &U4P1Rule{NewBaseRule(4, 1, "v3",
		[]string{"DialogueNodeEvent:88:0"},
		nil,
		WithTriggerMatchers(soilKeyClose),
	)}
}

func (r *U4P1Rule) Evaluate(ctx context.Context, db *mongo.Database, game, userID string, ec EvalContext) (Result, error) {
	helper := NewLogDataHelper(db, game)
	w := ec.Window

	const correctKey = "DialogueNodeEvent:88:5" // the water-table boundary answer (88:7 is the wrong one)

	hasCorrect, err := helper.HasEventInWindow(ctx, userID, correctKey, w)
	if err != nil {
		return Result{}, err
	}

	// Earliest Unit-4 puzzle start inside the window; the trigger is the close.
	startDoc, err := helper.FindEarliestByEventTypeAndData(ctx, userID, "Soil Key Puzzle", map[string]any{
		"Soil Key Puzzle Status": "Started",
		"Unit":                   UnitScene(4),
	}, w)
	if err != nil {
		return Result{}, err
	}

	var duration *float64 // nil = no measured completion time
	if startDoc != nil && !startDoc.ServerTimestamp.IsZero() && !ec.EndTime.IsZero() {
		d := ec.EndTime.Sub(startDoc.ServerTimestamp).Seconds()
		duration = &d
	}

	score := 0.0
	if hasCorrect {
		score += 0.5
	}
	durationBonus := 0.0
	if duration != nil {
		switch {
		case *duration > 0 && *duration <= 30:
			durationBonus = 1.0
		case *duration > 30 && *duration <= 90:
			durationBonus = 0.5
		}
	}
	score += durationBonus

	mistakeCount := int64(0)
	if !hasCorrect {
		mistakeCount = 1
	}
	var durationMetric any
	if duration != nil {
		durationMetric = *duration
	}
	metrics := map[string]any{
		"hasCorrectAnswer":   hasCorrect,
		"puzzleDurationSecs": durationMetric,
		"durationBonus":      durationBonus,
		"score":              score,
		"mistakeCount":       mistakeCount,
	}
	if score >= 1 {
		return PassedWithMetrics(metrics), nil
	}

	choicePhrase := "chose 'it's any water found underground' instead of the correct answer on"
	if hasCorrect {
		choicePhrase = "answered correctly"
	}
	durationPhrase := "has no measured completion time for"
	if duration != nil {
		durationPhrase = fmt.Sprintf("took %d seconds to solve", JSRound(*duration))
	}
	return FlaggedWith(metrics, Reason{
		Code: "SCORE_BELOW_THRESHOLD",
		Variables: map[string]any{
			"choice_phrase":   choicePhrase,
			"duration_phrase": durationPhrase,
		},
	}), nil
}
