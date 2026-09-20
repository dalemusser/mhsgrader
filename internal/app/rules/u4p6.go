package rules

import (
	"context"
	"strings"

	"go.mongodb.org/mongo-driver/mongo"
)

// U4P6Rule — Desert Delicacies: the soil chosen for each garden box.
//
// Spec: mhsgrading/grading-logic/mhs-unit4-point6-grading.md (2026-09).
// Window: latest questActiveEvent:41 before the trigger (exclusive) →
// questFinishEvent:56 (inclusive).
// Box score: +1 per box whose latest cameraPlaced soil is right (box "0"
// Gravel, "1" Sand, "2" Clay). Dialogue fallback (box ids have shifted
// between builds): count of correct-soil feedback 92:61 after the latest
// review start 92:33 in the window (whole window when none), capped at 3.
// Final = max(box score, dialogue score); green iff final >= 2.
// Reason WRONG_SOIL_SELECTED: wrong_box_number, wrong_box_summary (built
// from the box check).
type U4P6Rule struct{ BaseRule }

func NewU4P6Rule() *U4P6Rule {
	return &U4P6Rule{NewBaseRule(4, 6, "v3",
		[]string{"questActiveEvent:41"},
		[]string{"questFinishEvent:56"},
	)}
}

func (r *U4P6Rule) Evaluate(ctx context.Context, db *mongo.Database, game, userID string, ec EvalContext) (Result, error) {
	helper := NewLogDataHelper(db, game)
	w := ec.Window

	const (
		reviewStartKey     = "DialogueNodeEvent:92:33" // Dani starts reviewing the boxes
		correctFeedbackKey = "DialogueNodeEvent:92:61" // correct-soil feedback, once per box
	)

	// Expected soil per box (zero-indexed boxId from the game), in box order.
	boxes := []struct{ id, expected, label string }{
		{"0", "Gravel", "the first box"},
		{"1", "Sand", "the second box"},
		{"2", "Clay", "the third box"},
	}

	metrics := map[string]any{}
	boxScore := int64(0)
	var wrongParts []string
	for _, b := range boxes {
		entry, err := helper.FindLatestByEventTypeAndData(ctx, userID, "TerasGardenBox",
			map[string]any{"actionType": "cameraPlaced", "boxId": b.id}, w)
		if err != nil {
			return Result{}, err
		}
		actual := ""
		if entry != nil {
			actual, _ = entry.Data["soilType"].(string)
		}
		correct := actual == b.expected
		switch {
		case correct:
			boxScore++
		case actual != "":
			wrongParts = append(wrongParts, b.label+" (chose "+actual+", needs "+b.expected+")")
		default:
			wrongParts = append(wrongParts, b.label+" (no camera placement recorded, needs "+b.expected+")")
		}
		metrics["box"+b.id+"SoilType"] = actual
		metrics["box"+b.id+"Correct"] = correct
	}

	// Dialogue-feedback fallback: 92:61 in the latest review round.
	latestReview, err := helper.LatestEventInWindow(ctx, userID, []string{reviewStartKey}, w)
	if err != nil {
		return Result{}, err
	}
	feedbackWindow := w
	if latestReview != nil {
		feedbackWindow = w.Sub(latestReview.ID, w.EndID)
	}
	feedbackCount, err := helper.CountEventInIDWindow(ctx, userID, correctFeedbackKey, feedbackWindow)
	if err != nil {
		return Result{}, err
	}
	dialogueScore := min(feedbackCount, 3)

	finalScore := max(boxScore, dialogueScore)
	wrongBoxes := int64(len(wrongParts))

	metrics["boxScore"] = boxScore
	metrics["dialogueScore"] = dialogueScore
	metrics["score"] = finalScore
	metrics["mistakeCount"] = wrongBoxes
	if finalScore >= 2 {
		return PassedWithMetrics(metrics), nil
	}
	return FlaggedWith(metrics, Reason{
		Code: "WRONG_SOIL_SELECTED",
		Variables: map[string]any{
			"wrong_box_number":  wrongBoxes,
			"wrong_box_summary": strings.Join(wrongParts, " and "),
		},
	}), nil
}
