// internal/app/rules/ea.go
// Embedded Assessment (EA) checkpoint scores — the numbers the end-of-game
// ceremony branches on, computed by the rules alongside the colour and stored
// on every finished attempt (docs/updates/ea-scores.md; the team decisions in
// mhsgrading/docs/ea-scores-team-questions-2026-09.md).
//
// A rule emits a checkpoint only when it can score it from a well-bounded
// window: an absent checkpoint means "unknown" to the ceremony (the gentle
// line), never a fabricated 0.
package rules

import "sort"

// EAScore is one checkpoint's score and maximum.
type EAScore struct {
	Score float64
	Max   float64
}

// Checkpoint ids, verbatim from the EA working document.
const (
	EAU2C2 = "U2.C2" // Find Toppo
	EAU2C3 = "U2.C3" // Find Tera + Find Aryn (D1)
	EAU2C5 = "U2.C5" // Classify argument components
	EAU2C7 = "U2.C7" // Argue which watershed is bigger
	EAU3C1 = "U3.C1" // Placement of crates
	EAU3C5 = "U3.C5" // Plant superfruit seeds
	EAU4C6 = "U4.C6" // Tera's garden boxes
	EAU5C3 = "U5.C3" // What happened to the water?
	EAU5C4 = "U5.C4" // Solar still
)

// EACheckpoint describes an implemented checkpoint: the unit it belongs to,
// its maximum, and the progress point whose rule emits it.
type EACheckpoint struct {
	ID      string
	Unit    int
	Max     float64
	PointID string
}

// EACheckpoints lists every checkpoint the grader emits today (the nine the
// ceremony's dialogue reads). The star totals (EAStars) are computed over this
// list, so adding a checkpoint here extends the totals automatically.
var EACheckpoints = []EACheckpoint{
	{EAU2C2, 2, 1, "u2p2"},
	{EAU2C3, 2, 3, "u2p3"},
	{EAU2C5, 2, 6, "u2p5"},
	{EAU2C7, 2, 3, "u2p7"},
	{EAU3C1, 3, 3, "u3p1"},
	{EAU3C5, 3, 4, "u3p5"},
	{EAU4C6, 4, 3, "u4p6"},
	{EAU5C3, 5, 3, "u5p3"},
	{EAU5C4, 5, 1.5, "u5p4"},
}

// EACheckpointsForUnit returns the implemented checkpoints of a unit.
func EACheckpointsForUnit(unit int) []EACheckpoint {
	var out []EACheckpoint
	for _, c := range EACheckpoints {
		if c.Unit == unit {
			out = append(out, c)
		}
	}
	return out
}

// EAStarUnits are the units with a star row (the EA document's "Unit Summary
// Scores for Dashboard" table starts at Unit 2).
var EAStarUnits = []int{2, 3, 4, 5}

// ---- banding helpers (the EA document's point tables) ----------------------

// EAHelpBand1 — "1 point: help dialog once or less; ½: twice; 0: more"
// (U2.C2 Find Toppo, max 1).
func EAHelpBand1(helpCount int64) float64 {
	switch {
	case helpCount <= 1:
		return 1
	case helpCount == 2:
		return 0.5
	default:
		return 0
	}
}

// EAHelpBand15 — "1½: once or less; 1: two or three times; ½: four; 0: more"
// (each of the Tera and Aryn searches in U2.C3, max 1.5 each).
func EAHelpBand15(helpCount int64) float64 {
	switch {
	case helpCount <= 1:
		return 1.5
	case helpCount <= 3:
		return 1
	case helpCount == 4:
		return 0.5
	default:
		return 0
	}
}

// EAAttemptsBand3 — "3 points: correct within 3 attempts; 2: 4 attempts;
// 1: 5 attempts; 0: more, or never correct" (U2.C7, U5.C3, max 3).
// attempts counts the successful submission too (wrong submissions + 1).
func EAAttemptsBand3(attempts int64, succeeded bool) float64 {
	if !succeeded {
		return 0
	}
	switch {
	case attempts <= 3:
		return 3
	case attempts == 4:
		return 2
	case attempts == 5:
		return 1
	default:
		return 0
	}
}

// ---- solar still (U5.C4) ---------------------------------------------------

// EASolarStillOutcome is what one conversation-106 outcome node says about
// the settings the student chose (from the 2026-09-21 dialogue export, where
// each node's title names its combination). Half a point each for Tilted Out,
// Uncovered (the X mark: no extra converting) and Cold (D6).
type EASolarStillOutcome struct {
	Roof      string // "Tilted In" | "Flat" | "Tilted Out"
	Uncovered bool
	Cold      bool
}

// Score is the checkpoint score for the outcome.
func (o EASolarStillOutcome) Score() float64 {
	s := 0.0
	if o.Roof == "Tilted Out" {
		s += 0.5
	}
	if o.Uncovered {
		s += 0.5
	}
	if o.Cold {
		s += 0.5
	}
	return s
}

// EASolarStillOutcomes maps every outcome node (eventKey) to its settings.
var EASolarStillOutcomes = map[string]EASolarStillOutcome{
	"DialogueNodeEvent:106:4":  {"Tilted In", false, true},
	"DialogueNodeEvent:106:25": {"Tilted In", false, false},
	"DialogueNodeEvent:106:26": {"Flat", false, true},
	"DialogueNodeEvent:106:27": {"Flat", false, false},
	"DialogueNodeEvent:106:28": {"Tilted Out", false, true},
	"DialogueNodeEvent:106:29": {"Tilted Out", false, false},
	"DialogueNodeEvent:106:30": {"Tilted In", true, false},
	"DialogueNodeEvent:106:31": {"Flat", true, false},
	"DialogueNodeEvent:106:32": {"Tilted Out", true, false},
	"DialogueNodeEvent:106:33": {"Tilted In", true, true},
	"DialogueNodeEvent:106:34": {"Flat", true, true},
	"DialogueNodeEvent:106:35": {"Tilted Out", true, true},
}

// EASolarStillKeys returns the outcome node keys in a stable order.
func EASolarStillKeys() []string {
	keys := make([]string, 0, len(EASolarStillOutcomes))
	for k := range EASolarStillOutcomes {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// ---- stars -----------------------------------------------------------------

// EAStarsForUnit bands a unit's total over the implemented checkpoints into
// 1–3 stars (D7, interim until the team confirms the per-unit totals): three
// stars at ≥ 83 % of the implemented maximum, two at ≥ 57 %, otherwise one —
// the shares the EA document's own band table implies. A checkpoint with no
// score contributes 0 to the total. ok is false when the unit has no
// implemented checkpoints (no star row).
func EAStarsForUnit(unit int, scores map[string]EAScore) (stars int, ok bool) {
	cps := EACheckpointsForUnit(unit)
	if len(cps) == 0 {
		return 0, false
	}
	total, max := 0.0, 0.0
	for _, c := range cps {
		max += c.Max
		if s, found := scores[c.ID]; found {
			total += s.Score
		}
	}
	if max <= 0 {
		return 0, false
	}
	share := total / max
	switch {
	case share >= 0.83:
		return 3, true
	case share >= 0.57:
		return 2, true
	default:
		return 1, true
	}
}

// eaOne is a one-checkpoint EAScores map for a Result.
func eaOne(id string, score, max float64) map[string]EAScore {
	return map[string]EAScore{id: {Score: score, Max: max}}
}
