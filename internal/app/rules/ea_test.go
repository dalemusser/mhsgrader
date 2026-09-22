package rules

import (
	"math"
	"testing"
)

func TestEAHelpBands(t *testing.T) {
	// U2.C2: 1 / ½ / 0
	for count, want := range map[int64]float64{0: 1, 1: 1, 2: 0.5, 3: 0, 9: 0} {
		if got := EAHelpBand1(count); got != want {
			t.Errorf("EAHelpBand1(%d) = %v, want %v", count, got, want)
		}
	}
	// U2.C3 per search: 1½ / 1 / ½ / 0
	for count, want := range map[int64]float64{0: 1.5, 1: 1.5, 2: 1, 3: 1, 4: 0.5, 5: 0, 12: 0} {
		if got := EAHelpBand15(count); got != want {
			t.Errorf("EAHelpBand15(%d) = %v, want %v", count, got, want)
		}
	}
}

func TestEAAttemptsBand3(t *testing.T) {
	for attempts, want := range map[int64]float64{1: 3, 2: 3, 3: 3, 4: 2, 5: 1, 6: 0, 20: 0} {
		if got := EAAttemptsBand3(attempts, true); got != want {
			t.Errorf("EAAttemptsBand3(%d, true) = %v, want %v", attempts, got, want)
		}
	}
	if got := EAAttemptsBand3(1, false); got != 0 {
		t.Errorf("never succeeded must score 0, got %v", got)
	}
}

func TestEASolarStillOutcomes(t *testing.T) {
	want := map[string]float64{
		"DialogueNodeEvent:106:4":  0.5, // Tilted In, Covered, Cold
		"DialogueNodeEvent:106:25": 0,   // Tilted In, Covered, Hot
		"DialogueNodeEvent:106:26": 0.5, // Flat, Covered, Cold
		"DialogueNodeEvent:106:27": 0,   // Flat, Covered, Hot
		"DialogueNodeEvent:106:28": 1,   // Tilted Out, Covered, Cold
		"DialogueNodeEvent:106:29": 0.5, // Tilted Out, Covered, Hot
		"DialogueNodeEvent:106:30": 0.5, // Tilted In, Uncovered, Hot
		"DialogueNodeEvent:106:31": 0.5, // Flat, Uncovered, Hot
		"DialogueNodeEvent:106:32": 1,   // Tilted Out, Uncovered, Hot
		"DialogueNodeEvent:106:33": 1,   // Tilted In, Uncovered, Cold
		"DialogueNodeEvent:106:34": 1,   // Flat, Uncovered, Cold
		"DialogueNodeEvent:106:35": 1.5, // the success node: every setting right
	}
	if len(EASolarStillOutcomes) != len(want) {
		t.Fatalf("outcome map has %d nodes, want %d", len(EASolarStillOutcomes), len(want))
	}
	for key, w := range want {
		o, ok := EASolarStillOutcomes[key]
		if !ok {
			t.Errorf("%s missing", key)
			continue
		}
		if got := o.Score(); got != w {
			t.Errorf("%s (%+v) scores %v, want %v", key, o, got, w)
		}
	}
	// The colour rule's failure sets must cover every node but the success one.
	if keys := EASolarStillKeys(); len(keys) != 12 || keys[0] != "DialogueNodeEvent:106:25" {
		t.Errorf("EASolarStillKeys = %v", keys)
	}
}

func TestEACheckpointsAndStars(t *testing.T) {
	if len(EACheckpoints) != 9 {
		t.Fatalf("expected the nine ceremony checkpoints, got %d", len(EACheckpoints))
	}
	seen := map[string]bool{}
	for _, c := range EACheckpoints {
		if seen[c.ID] {
			t.Errorf("duplicate checkpoint %s", c.ID)
		}
		seen[c.ID] = true
		if c.Max <= 0 || c.Unit < 2 || c.Unit > 5 || c.PointID == "" {
			t.Errorf("bad checkpoint %+v", c)
		}
	}
	// Unit 2 implemented maximum: 1 + 3 + 6 + 3 = 13.
	total := 0.0
	for _, c := range EACheckpointsForUnit(2) {
		total += c.Max
	}
	if total != 13 {
		t.Fatalf("unit 2 implemented max = %v, want 13", total)
	}

	full := map[string]EAScore{EAU2C2: {1, 1}, EAU2C3: {3, 3}, EAU2C5: {6, 6}, EAU2C7: {3, 3}}
	if s, ok := EAStarsForUnit(2, full); !ok || s != 3 {
		t.Errorf("full marks → 3 stars, got %d (ok=%v)", s, ok)
	}
	// 83 % boundary: 10.79/13 = 0.83
	if s, _ := EAStarsForUnit(2, map[string]EAScore{EAU2C5: {6, 6}, EAU2C7: {3, 3}, EAU2C3: {1.79, 3}}); s != 3 {
		t.Errorf("83 %% → 3 stars, got %d", s)
	}
	// 57 % boundary: 7.41/13 = 0.57
	if s, _ := EAStarsForUnit(2, map[string]EAScore{EAU2C5: {6, 6}, EAU2C7: {1.41, 3}}); s != 2 {
		t.Errorf("57 %% → 2 stars, got %d", s)
	}
	if s, _ := EAStarsForUnit(2, map[string]EAScore{EAU2C5: {6, 6}, EAU2C7: {1.4, 3}}); s != 1 {
		t.Errorf("just under 57 %% → 1 star, got %d", s)
	}
	// Missing checkpoints count as 0 toward the total, not as unknown.
	if s, ok := EAStarsForUnit(2, map[string]EAScore{}); !ok || s != 1 {
		t.Errorf("no scores → 1 star (ok), got %d (ok=%v)", s, ok)
	}
	// Unit 1 has no star row.
	if _, ok := EAStarsForUnit(1, full); ok {
		t.Errorf("unit 1 must have no star row")
	}
	// Unit 5: max 4.5; 4.5/4.5 → 3; 3/4.5 = 0.667 → 2; 1.5/4.5 → 1.
	for score, want := range map[float64]int{4.5: 3, 3: 2, 1.5: 1} {
		got, _ := EAStarsForUnit(5, map[string]EAScore{EAU5C3: {math.Min(score, 3), 3}, EAU5C4: {math.Max(score-3, 0), 1.5}})
		if got != want {
			t.Errorf("unit 5 total %v → %d stars, want %d", score, got, want)
		}
	}
}
