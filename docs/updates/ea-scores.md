# EA Checkpoint Scores — Implementation Brief

*Written 2026-08-03 for a future mhsgrader work cycle. Self-contained: everything
needed to implement is here or at the referenced paths.*

## Why

The MHS end-of-game ceremony (`mhs-gameplay-end` repo, deployed under
`/mhs/end/vX.Y.Z/` on the CDN) plays different dialog variants depending on a
student's **Embedded Assessment (EA) checkpoint scores** — e.g. Jasper's happy
"you found our missing team members quickly" line requires "EA score ≥ 2 on
U2.C2 and U2.C3." Decision (Dale, 2026-08-03): **mhsgrader computes EA scores at
grade time and stores them in the grade documents** — one grading brain, stored
and auditable — rather than deriving them at read time in stratahub.

Downstream flow once this lands: stratahub adds a session-authenticated,
self-only JSON endpoint (`GET /missionhydrosci/api/ea-scores`) that reads the
grades DB and serves the contract below; the ceremony's resolver picks dialog
variants from it. The ceremony (script v2, 2026-08-04) needs the **nine
checkpoints** below, but the mechanism should generalize — the EA working doc
defines 23 checkpoints (U1.C3 … U5.C4).

## Output contract (what stratahub must be able to serve)

```json
{
  "game": "mhs",
  "user_id": "665f1a2b3c4d5e6f7a8b9c0d",
  "generatedAt": "2026-08-03T12:00:00Z",
  "currentUnit": "unit5",
  "items": {
    "U2.C2": { "score": 1.0, "max": 1.0 },
    "U2.C3": { "score": 0.5, "max": 1.5 }
  },
  "stars": { "unit2": 2, "unit3": 3, "unit4": 1, "unit5": 2 }
}
```

(`stars` = per-unit dashboard star ratings for the ending pop-up — see note 4.)

- Keys are EA checkpoint IDs exactly as the ceremony references them.
- An item **absent** from `items` means never attempted / not graded. **Never
  fabricate a 0** — the ceremony treats missing as *unknown* and falls back to
  the gentler dialog variant; a fabricated 0 would instead assert poor
  performance.
- Scores are numeric; halves and thirds occur.
- Canonical fixture examples (the shapes the ceremony is tested against):
  `mhs-gameplay-end/test-profiles/*.json`.

## Storage recommendation

Add an `eaScores` field to each `Grade` entry (per attempt) in
`internal/app/store/progressgrades/store.go`:

```go
type Grade struct {
    // … existing fields …
    EAScores map[string]EAScore `bson:"eaScores,omitempty"` // "U2.C2" -> {score, max}
}
type EAScore struct {
    Score float64 `bson:"score"`
    Max   float64 `bson:"max"`
}
```

Per-attempt (rather than one map on `UserGrades`) keeps it auditable and
replay-safe; the stratahub reader takes the **latest attempt** per progress
point, mirroring how the dashboard reads grades today
(`stratahub/internal/app/features/mhsdashboard/summary.go` `loadPlayerGrades`).
BSON only — no JSON tags, same as the rest of the doc (nothing serializes these
structs to HTTP directly).

Most rules map 1:1 (one rule → one checkpoint), so computing inside each rule's
`Evaluate` (`internal/app/rules/u2p2.go` etc.) alongside the existing
status/metrics is the natural seam — the raw counts are already in scope there.

## Verified crosswalk (rule source + EA working doc, checked 2026-08-03)

EA definitions: `mhscurriculum/docs/curriculum/Game Wide Docs/MHS 2.0 Embedded
Assessment Working Doc.md` (HTML table). Rule files:
`mhsgrader/internal/app/rules/`. **The join is by task semantics / dialogue-tag
lists (verified identical), not by number — progress-point numbers and
checkpoint numbers coincide only sometimes.**

| EA checkpoint | Task | Rule | Metrics stored today | EA score derivation | Max |
|---|---|---|---|---|---|
| **U2.C2** | Find Toppo | `u2p2.go` | `mistakeCount` (help-dialog triggers) | band: ≤1 → 1, =2 → ½, >2 → 0 | 1 |
| **U2.C3** | Find Tera | `u2p3.go` | `mistakeCount` | band: ≤1 → 1½, 2–3 → 1, =4 → ½, >4 → 0 | 1.5 |
| **U2.C5** | Classify Argument Components | `u2p5.go` | `posCount`, `mistakeCount`, **`score`** = pos − neg/3 | reuse `score` as-is (see note 1) | ~6 (note 1) |
| **U2.C7** | Argue Which Watershed Is Bigger | `u2p7.go` | `mistakeCount` (= wrong submissions), pass requires success event | attempts = mistakes + 1 when succeeded: ≤3 att → 3, 4 → 2, 5 → 1, else/never-succeeded → 0 | 3 |
| **U3.C1** | Placement of Crates | `u3p1.go` | **`count`** (correct placements), `mistakeCount` | EA score = `count` | 3 |
| **U3.C5** (see note 2) | Plant Superfruit Seeds | `u3p5.go` | `posCount`, `mistakeCount`, **`score`** = pos − 0.5·neg | reuse `score` as-is — formula matches EA doc exactly | 4 (note 2) |
| **U4.C6** | Tera's Garden Boxes | `u4p6.go` | **`score`** (0–3, +1 per correct soil), `box{N}SoilType/Correct` | reuse `score` as-is | 3 |
| **U5.C3** | Argumentation: What happened to the water? | `u5p3.go` | `mistakeCount` (negative dialogues) | same shape as U2.C7: attempts = mistakes + 1 → ≤3 att → 3, 4 → 2, 5 → 1, else 0 (ceremony conditions on `> 0`) | 3 |
| **U5.C4** (see note 3) | Solar Still Activity | `u5p4.go` | success + zero-tolerance pass/flag | per-selection: ½ pt each for Tilted Out / X no-extra-converting / cold glass roof (needs new event detection — note 3) | 1.5 |

### Note 1 — U2.C5's EA doc is internally inconsistent

The EA doc's *points* column says raw "+1 per correct, −⅓ per incorrect" (which
is exactly what `u2p5.go` computes), but its *formula* column says a
**proportional** version (`correct/total − 0.33·incorrect/total`). The ceremony
conditions on `U2.C5 ≥ 2`, which only makes sense against the **raw** form —
implement the raw form (i.e. reuse the rule's existing `score`). Max is not
crisply defined (22 positive log keys exist; the doc says "need 6 correct for
completion") — use **6** for `max` unless the designers say otherwise; only
`score` vs. the threshold matters to the ceremony.

### Note 2 — U3.C5 numbering (RESOLVED 2026-08-04)

The designers confirmed the superfruit-garden checkpoint is **U3.C5** (the v1
script's "U3.C4" was a typo). The ceremony definition and test fixtures already
emit/expect `U3.C5` — mhsgrader should emit the same key. The EA doc says 4
garden boxes / 4 pts max; the ceremony's happy-variant threshold is ≥3,
consistent with the rule's pass bar.

Also resolved: "≥2 on U2.C2 and U2.C3" means the **sum** (the ceremony resolver
sums — mhsgrader just emits the two scores separately), and missing scores play
the generic variant (ceremony-side; reinforces "never fabricate zeros").

### Note 3 — Solar still: RESOLVED (2026-08-05), needs new detection

The checkpoint is **U5.C4** (the script's "C5" was a typo, designer-confirmed),
and the EA doc (v2, vendored at `mhs-gameplay-end/designer-content/`) now
reflects the one-chance build: **three ½-point selections** — Tilted Out, the X
mark for no extra converting, and cold for the glass-roof temperature — max
**1.5**. The ceremony conditions on `U5.C4 > 0` (any correct selection earns
Aryn's happy variant). Implementation caveat: the existing `u5p4` rule only
detects overall success/failure via feedback dialogue nodes (correct 106:35,
incorrect 106-4,25…34) — the per-selection EA score needs detection of the
three individual selections, which likely means new log-event analysis, not a
metrics passthrough.

### Note 4 — Per-unit stars (NEW requirement, 2026-08-05)

The ceremony's ending shows a star pop-up (per-unit 0–3 stars), and the
dashboard uses the same bands. Stars are **unit totals across ALL of that
unit's checkpoints** (not just the ceremony's subset) banded per the EA doc's
"Unit Summary Scores for Dashboard" table:

| Unit | 1 star | 2 stars | 3 stars |
|---|---|---|---|
| 2 Topography | 0–8.9 | 9–12.49 | 12.50–15 |
| 3 Surface Water | 0–6.9 | 7–9.9 | 10–12 |
| 4 Groundwater | 0–8.4 | 8.5–12.4 | 12.5–16 |
| 5 Atmospheric Water | 0–5.99 | 6–8.99 | 9–10.5 |

This means the EA computation must eventually cover **every** checkpoint in
units 2–5 (23 in the EA doc), not only the ceremony's nine. The contract adds:

```json
"stars": { "unit2": 2, "unit3": 3, "unit4": 1, "unit5": 2 }
```

(absent key = unit unplayed; the ceremony shows that row with zero filled
stars). Canonical fixture examples updated in `mhs-gameplay-end/test-profiles/`.

### Note 5 — U2.C3 composition (OPEN)

A teammate reads U2.C3 as aggregating the Tera AND Aryn searches (1.5 each,
max 3 → C2+C3 max 4); the EA doc's row says "Find Tera", max 1.5 (→ combined
max 2.5), and no Aryn checkpoint exists in the doc. Ask pending with Eric/Erin
(`mhs-gameplay-end/docs/designer-question-u2c3.md`). The ceremony sums C2+C3
against 2 either way — only the banding/max here changes with the answer.

## Backfill

Existing students have grades but no `eaScores`. The grader is a cursor-driven
poll daemon (`grader_state` collection, `lastSeenId`), so new logdata will get
EA scores organically, but historical grades need a one-time reprocess:

- Add a backfill command/mode that re-runs `Evaluate` per (user, rule) over the
  stored attempt windows (`Grade.StartTime`/`EndTime` are already stored) and
  writes `eaScores` into the existing attempt entries — do **not** append new
  attempts or change `status`/`computedAt` semantics.
- Pre-cutover logdata keyed by `login_id` (before May 2026) is out of scope —
  the ceremony serves current students only.
- DocumentDB constraints apply (see `CLAUDE.md`): no `$facet`, and single-field
  indexes don't satisfy compound sorts — check any new query's index needs.

## Implementation checklist

1. `EAScore` type + `eaScores` field on `Grade` (progressgrades store).
2. Per-rule EA computation in the rules above (band tables for
   u2p2/u2p3/u2p7/u5p3; passthrough for u2p5/u3p1/u3p5/u4p6; solar still on
   hold per note 3). A small shared helper for banding keeps the tables
   declarative and testable.
3. Unit tests per rule: band edges (e.g. mistakes = 1, 2, 3 for U2.C2), the
   never-succeeded case for U2.C7, and absence semantics (no attempt → no entry).
4. Backfill command + run it on prod after deploy.
5. Update `docs/statistics_and_data_collection.md` (documents every rule's
   metrics) with the new field.
6. Hand off to stratahub: the `/missionhydrosci/api/ea-scores` endpoint (Phase 5
   of `mhs-gameplay-end/docs/implementation-plan.md`) — reader = latest attempt
   per point, merged across points into the contract above.

## References

- Ceremony plan & contract: `mhs-gameplay-end/docs/implementation-plan.md` (§2.1, Phase 4)
- Designer questions (Q1–Q3 affect keys/semantics): `mhs-gameplay-end/docs/designer-questions.md`
- Ceremony resolver (consumer semantics): `mhs-gameplay-end/lib/resolver.js`
- Canonical fixtures: `mhs-gameplay-end/test-profiles/*.json`
- EA definitions: `mhscurriculum/docs/curriculum/Game Wide Docs/MHS 2.0 Embedded Assessment Working Doc.md`
- Grade doc shape: `mhsgrader/internal/app/store/progressgrades/store.go`
- Existing reader to mirror: `stratahub/internal/app/features/mhsdashboard/summary.go`

## Status 2026-09-22 — G1 implemented

The nine ceremony checkpoints are computed by their rules (`Result.EAScores`,
stored as `Grade.eaScores` per finished attempt) and the per-unit stars as
`eaStars` on the user document (interim share-of-maximum rule), see
`internal/app/rules/ea.go` and `docs/statistics_and_data_collection.md`
"EA checkpoint scores". Backfill = wipe and replay (`aws_reset.sh` + deploy),
not a separate command: the colour rules are unchanged. The remaining fourteen
checkpoints (G2) extend `EACheckpoints` and their rules the same way.

## Addendum 2026-09-21 — decisions for the first playable ceremony

The team-facing decisions and questions are in
`mhsgrading/docs/ea-scores-team-questions-2026-09.md` (answers expected in an
`…-answers.md` beside it). They supersede notes 3 and 5 above:

- **U2.C3 = Tera + Aryn (max 3).** `u2p3.go` already tracks both key sets and
  the Aryn start (`18:231`); emit the two bandings separately and sum them.
- **U5.C4 from the outcome node** (`106:4 … 106:35`): each node's title names
  the roof/cover/temperature combination; ½ each for Tilted Out, Uncovered,
  Cold; score the first submission.
- **All checkpoints in the EA document are computed** (stars need unit
  totals); the crosswalk with derivations is in the team document §2.
- **Stars**: share of the unit's implemented maximum until the team confirms
  the totals (≥ 83 % three, ≥ 57 % two, else one; no graded checkpoint → no
  entry). Unit 1 has no star row.
- **Critique Jasper's argument** is held until the team names the
  conversation; **U3.C2** reuses the rule's 5-point score.
- The stratahub plan that consumes all this:
  `stratahub/docs/mission-hydrosci/mhs-end-ceremony-plan.md`.
