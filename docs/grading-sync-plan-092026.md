# mhsgrader ↔ mhsgrading sync plan (September 2026)

*Written 2026-09-20. Brings the Go grader up to the September 2026 grading
specification in `mhsgrading/`, then the StrataHub MHS Dashboard up to the new
grade shape. Source comparison: all 26 `grading-logic/*.md` docs and their Python
transcriptions against `internal/app/rules/*.go`.*

## 0. Status / how to resume

- **2026-09-20 (evening):** Phase A done — engine (per-rule windows, event matchers,
  reasons with variables, replay harness, `--once`), all 26 rules rewritten to the
  spec (v3), replay harness at **26/26 colours and 26/26 reason codes + variables on
  all five fixtures**; catalog extractor (`cmd/mhsreasoncodes`) and dashboard rendering
  (Phase B1–B3) committed in stratahub. Dale's answers to §7: windows per script (Q1),
  wipe + regrade approved (Q2), scripts as validated (Q3), U3P2 start decided by us
  (Q4, `questActiveEvent:17`), AI pop-up later (Q5), EA scores out (Q6), doc
  inconsistencies listed for the grading team (Q7) → the shared repo: `mhsgrading/docs/grading-team-questions-2026-09.md`; corrections already made: `mhsgrading/docs/grading-doc-changes-2026-09-21.md`.
- **2026-09-20 23:27 UTC:** production grades wiped (664 documents) and the v3 grader
  deployed; stratahub deployed 23:27 UTC. The replay stalled at 2026-08-17 on two log
  records whose `data` is a string (see the questions doc §F); tolerant decoding was
  deployed 00:10 UTC and the replay resumed from the same cursor (no second wipe).
- **2026-09-21 00:13 UTC:** regrade complete (cursor at the last trigger event of
  2026-09-18, zero errors after the fix). 745 students graded; latest grades: 7,123
  passed, 2,517 flagged (2,390 with reasons; the 127 without are the spec's no-code
  states, almost all on spring-2026 builds), 1,428 active (312 of them after a finished
  attempt, shown as the finished grade). Replay of 2.65 M logs took ~15 min.
- **2026-09-21:** Dale confirmed the review pop-up on the live dashboard (screenshots
  of U2P2 and U4P2). Instructor messages reworded without em-dashes (mhsgrading +
  catalog regenerated + stratahub redeployed). Documentation inconsistencies with a
  clear answer fixed in mhsgrading and recorded in
  `mhsgrading/docs/grading-doc-changes-2026-09-21.md`; both Python suites and the Go
  harness still 26/26. Dale shares that file and
  `mhsgrading/docs/grading-team-questions-2026-09.md` with Wenyi.

### How to resume

Everything shipped is in production; the cycle is paused, not mid-task. Open items,
in likely order:

1. **Grading-team answers** to `mhsgrading/docs/grading-team-questions-2026-09.md`.
   Any change to a script → edit the markdown, its Python transcription and the Go
   rule together (`mhsgrading/ai/context.md` "Keeping the three copies in sync"),
   run both Python suites and `go test ./internal/app/grader/ -run TestFixtureReplay`,
   bump the rule to `_v4`, deploy with `mhsgrader_update/aws_update.sh`; a colour
   change on already-graded students needs `aws_reset.sh` + redeploy (15-minute
   replay, dashboard cells refill as it runs). A wording-only change → regenerate the
   catalog (`go run ./cmd/mhsreasoncodes -o ../stratahub/internal/app/resources/mhs_reason_codes.json`)
   and deploy stratahub only.
2. **B4 — per-point AI summary button** inside the pop-up (Dale: later). Inputs per
   point are listed in the mhsgrading README ("Generating teacher feedback"); the
   existing whole-student summary path is `mhsdashboard/summary.go`.
3. **B5 — teacher guide** note on the Progress-view pop-up (the guide only covers the
   Devices view); sources under `stratahub/docs/mission-hydrosci-teacher-guide/`.
4. **Debug timeline key annotations** (`stratahub/internal/app/resources/mhs_grading_rules.json`)
   were hand-updated for the changed points; the evaluated-key lists for the other
   points still reflect March. Generating that file from the Go rules would keep it
   honest.
5. **EA checkpoint scores** (`docs/updates/ea-scores.md`): the nine ceremony
   checkpoints and the interim stars shipped 2026-09-22 (G1); the other fourteen
   checkpoints (G2) are next, driven by the team's answers in
   `mhsgrading/docs/ea-scores-team-questions-2026-09.md`.
6. Watch for the **spec states without a reason code** (questions doc A9) and the
   **string-payload log records** (questions doc F) in the grading team's replies.

Local verification needs a running MongoDB (`mongod` via Homebrew) and the sibling
`mhsgrading` checkout; the Python suites need `pyyaml` (a venv is fine).

## 1. Where things stand

Headline: of 26 points, **14 need a colour-affecting change** (keys, thresholds,
operators, anchors or a new scoring path), **12 keep their colour logic** but need
the new reason-code names and message variables (7 of those need only metric
renames or nothing). Four points are **wrong in production today** independent of
the spec update: U3P4 (always flagged), U4P1 (cannot award its duration points),
U3P2 (phantom "active" attempt hides the grade), U4P2 (intermittent false yellow).

### Baseline: current mhsgrader binary replayed over two validated fixtures (2026-09-20, local MongoDB)

Method: mongoimport the mhsgrading playthrough dump into a scratch `logdata`, run the
current grader (unchanged code, default config) until its cursor reached the end,
compare the latest grade per point with `rubric-validation/config/fixtures.yaml`.

| Fixture | Build | Expected | Current grader | Mismatches |
|---|---|---|---|---|
| 09-14-26-3 (imperfect run) | 20260914- | 3 green / 23 yellow | 24 / 26 match | U3P2 shows **active** (2nd attempt opened after the end); U5P1 **green**, spec says yellow (rule tightened 2026-09-06 to zero negatives incl. the 100:39 offer node) |
| 09-03-26-2 (clean run) | 20260902-12353 | 26 green | 23 / 26 match | U3P2 **active** again; U3P4 and U4P2 **false yellow** (`MISSING_SUCCESS_NODE` with zero mistakes: the success/assist keys the Go code looks for no longer describe the current build) |

U3P2 root cause (seen in both fixtures): the documented start key `questFinishEvent:17`
fires *after* the end trigger `DialogueNodeEvent:11:34` (order in the log: 10:1 → 11:22 →
11:34 → questFinishEvent:17 → questFinishEvent:18). The grader grades attempt 1, then
the late start key appends an "active" attempt 2 that never closes, and the dashboard
shows the pencil instead of the flagged/passed cell. The spec's own production script
does not use that start key (it windows on the previous 11:34); the "Attempt Window
(Production)" block in the doc is the stale part.

Beyond colors, every stored reason code and metric name uses the March vocabulary
(`MISSING_SUCCESS_NODE`, `TOO_MANY_NEGATIVES`, `BAD_FEEDBACK`, `HIT_YELLOW_NODE`,
`mistakeCount`, …); none of the new instructor-message variables exist.

Also found: `MHSGRADER_BATCH_SIZE=5000` in the environment is rejected with
"batch_size must be at least 1" (the env value arrives as a string and the int
accessor yields 0). Config-file and default values work.

## 2. What the September spec changed (relative to the March code)

Two repos moved apart between March and September 2026:

- **mhsgrading** was reworked 2026-09-01 → 09-17 against builds 20260902 and 20260914:
  every point's "Production Script" was re-verified on real playthroughs, several
  key lists and thresholds changed (U2P3 `< 6`, U5P1 zero negatives, accepted-assist
  paths for the glyph puzzles, U4P6 dialogue fallback, U4P1/U4P2 anchored on the
  soil-key-puzzle event), and the reason codes were redesigned: one or two codes per
  point with a teacher-facing **Instructor Message** template whose `{variables}` a
  "Corresponding Script" computes. Five playthrough fixtures with expected colors,
  codes and variable values validate all of it (`rubric-validation/`,
  `reason-code-validation/`).
- **mhsgrader** rules were last changed 2026-03-23 (the 2026-09-20 commit is a
  `playerId → user_id` rename only). It still implements the March key lists and
  the March reason-code vocabulary, has no tests, and has no notion of message
  variables.

The dashboard reads whatever the grader stores, so it also shows the March
vocabulary through a fixed `reasonCodeToMessage` map and a generic
"Suggested Actions" list.

## 3. Design decisions (proposed)

**D1 — Window semantics: each rule computes the window its validated production
script uses.** The scripts fall into four patterns, so the window strategy becomes a
per-rule declaration that the evaluator executes: (i) previous occurrence of the END
trigger (exclusive) → this END (inclusive), zero ObjectID when none — U1P3, U2P1,
U2P4, U2P5, U2P7, U3P1, U3P2, U3P3, U3P5, U4P3, U4P5; (ii) latest START and latest
END, invalid (no grade) unless END follows START — U2P6, U3P4, U4P1, U4P4, U4P6 (strict),
U5P1–U5P3 (non-strict); (iii) latest START before this END, zero when none (today's
behaviour) — U2P2, U2P3, U4P2, U5P4; (iv) the U2P2/U2P3 colour scripts add a client
`timestamp` fence on top of (iii). Today the evaluator applies (iii) to every rule. All
patterns agree on a single playthrough; they differ on replays, and in three places
today's start keys are wrong outright (U3P4 and U4P1 always flag; U3P2 opens a phantom
attempt). The scripts are what five fixtures validated, so the grader should compute the
same intervals. Start keys stay for the "active" pencil state and the duration metrics.
The `timestamp` fence is implemented as written (the field decodes into `time.Time`;
the `_id` fence alone gives the same counts unless events were uploaded out of order).

**D2 — "Active after final" must not hide a grade.** Two guards: (a) correct any
start key that fires after its own end (U3P2 today; confirm the intended start
with the grading team), and (b) in the dashboard, when the latest attempt is
`active` but an earlier attempt is final, show the final grade's color with a
small "started again" marker instead of the pencil. (b) protects against the
same class of problem recurring after a build change.

**D3 — Reason codes stored the way the spec defines them.** Each flagged grade
stores `reasons: [{code, variables}]` in the doc's order (normally exactly one
code; the scripts are written to be mutually exclusive), plus the existing
`reasonCode` field set to the first code so nothing downstream breaks during the
transition. `variables` uses the spec's placeholder names verbatim
(`attempt_number`, `triggering_number`, `wrong_box_summary`, …) so the message
templates can be filled without any renaming layer. `metrics` keeps the raw
counts/scores for the analytics tab and future EA scoring; `mistakeCount` stays
defined for every rule as the point's headline error count.

**D4 — Message templates and teacher guidance live in a generated JSON, not in Go.**
A small generator reads `grading-logic/*.md` (the `## Reason Codes` sections; the
Python suite already has this parser in `rc_common.parse_reason_codes`) and emits
`mhs_reason_codes.json`: per point → per code → instructor-message template,
plus the point's Teacher Guidance. The file is committed into stratahub's embedded
resources (rendering side) and into mhsgrader (a test asserts every `{placeholder}`
in every template is a variable the corresponding rule emits, mirroring the Python
`placeholders:<CODE>` check). Wording changes then need a stratahub deploy only;
logic changes need a grader deploy; the markdown remains the single source.

**D5 — Triggers that are not eventKeys.** U4P1's end and U4P2's start are the Unit-4
`Soil Key Puzzle` event with `data."Soil Key Puzzle Status" = "Finished"` and
`data.Unit` matching `^Unit 4`. The registry/scanner gain a second trigger form
(eventType + data-field match); the scan filter becomes
`$or: [{eventKey: {$in: keys}}, {eventType: "Soil Key Puzzle", "data.Soil Key Puzzle Status": "Finished", "data.Unit": {$in: <unit-4 scene names>}}]`
with a `(game, eventType, _id)` index on logdata. An explicit `$in` of scene names
(from the same list the dashboard's `scene_to_unit` uses) is preferred over a regex
on DocumentDB; the regex stays documented as the spec's definition.

**D6 — Rule versions and the production regrade.** Every rule whose logic changes
becomes `_v3`. After deploy, run `--reset` (existing `aws_reset.sh`) and let the
grader replay all of `stratalog.logdata` — the design is replay-safe (cursor +
idempotent upserts). Expect minutes, not hours (36 triggers graded in ~100 ms
locally; scan is by the `(game, eventKey, _id)` index). The dashboard shows empty
cells until the replay passes each student, so do it at a quiet hour and
announce it. Pre-cutover `login_id` logs are not regraded (no compatibility path,
per the de-identification cutover).

**D7 — Test harness first.** Go tests replay the five mhsgrading fixtures through
the real scanner + evaluator against a local MongoDB (`mongod` and `mongoimport` are
installed here; tests skip when `MHSGRADER_TEST_MONGO_URI` is unreachable) and
assert, per point: color from `fixtures.yaml`, triggered codes and variable values
from `expectations.yaml`. This is the same evidence the Python suites use, so the Go
grader and the spec are held to one standard. Dumps are read from the sibling
`mhsgrading` checkout (`MHSGRADING_DIR`, default `../mhsgrading`); nothing is copied.
Per-rule unit tests cover threshold edges the fixtures do not reach (the eight code
paths the Python README lists as "verified only by transcription").

**D8 — Small fixes folded in.** Env-var override of integer settings (`batch_size`)
is broken; fix in config loading. Move `go.mod` to `go 1.25` per the workspace
standard. Refresh CLAUDE.md / ai/context.md / statistics doc (they still say
`playerId`, `log_entries`, 23 rules).

**Out of scope unless asked:** EA checkpoint scores (`docs/updates/ea-scores.md`,
a separate brief for the end-of-game ceremony). The new `metrics` are chosen so
that the EA bands can be derived later without another regrade.

## 4. Engine capabilities the new rules need (from the per-unit comparison)

| Need | Used by | Today |
|---|---|---|
| Window strategy declared per rule: previous-END → END; latest START + latest END with an end-after-start guard (strict or non-strict); start-before-END (current); optional client-`timestamp` fence | all points (see §5) | evaluator hard-codes start-before-END |
| Trigger and window anchors by eventType + data fields (with a prefix match on `data.Unit`) in the scanner and the evaluator | U4P1 end, U4P2 start | scanner matches `eventKey $in` only |
| Earliest event by eventKey inside a window | U2P3 phase split (`21:1`, `18:231`) | only latest-by-key exists |
| Latest event by eventKey inside a window, then a count in the sub-window after it | U4P6 dialogue fallback (`92:33` → `92:61`) | only eventType-based latest-in-window |
| Earliest event by eventType + data (+ prefix filter) in a window and a `serverTimestamp` duration to the anchor | U4P1 | pair lookup exists but without the Unit filter and anchored on last-in-window |
| Counts by key bucket (claim / reasoning / evidence, assist / negative) | U2P5, U2P7, U3P3, U4P2, U4P5, U5P3 | expressible with existing count helpers |
| String-valued and null-aware variables (`wrong_choice`, `failure_phrase`, `wrong_box_summary`, `duration_phrase`; unmeasured duration ≠ 0) | U2P6, U4P1, U4P6, U5P4 | metrics map accepts them; no rule emits them |
| "Window missing/invalid" outcome distinct from a graded yellow (spec: yellow cell, no code, dashboard shows not-reached) | U2P2, U2P3, U2P6, U3P4, U4P1, U4P4, U4P6, U5P1–P3 | evaluator always grades over (zero, END] |

## 5. Point-by-point: what changes

Verdicts: *no change* = colour and code already match (metric names may still change); *keys/threshold/operators* = colour logic edits; *reason layer* = colour identical, codes/variables new; *rewrite* = both; *new mechanism* = needs an engine capability from §4. Windows are the validated production scripts' anchors (D1). The per-unit comparison notes hold every key list and derivation; the markdown scripts remain the source of truth.

### Unit 1
| Point | Verdict | What changes in Go | Window (per script) | Reason codes → variables |
|---|---|---|---|---|
| U1P1 | no change | — | trigger only | — |
| U1P2 | no change | — | trigger only | — |
| U1P3 | rewrite | drop dead key `70:33`; yellow iff `70:25` in window; new `attempt_number` = count of {`70:25`, `70:7`} | prev `questActiveEvent:34` → latest (`_id`) | `WRONG_ARG_SELECTED` → `attempt_number` |
| U1P4 | no change | — | trigger only | — |

### Unit 2
| Point | Verdict | What changes in Go | Window (per script) | Reason codes → variables |
|---|---|---|---|---|
| U2P1 | reason layer | color identical (`68:29` success, 5 yellow nodes); assist keys {`68:24`,`68:26`,`68:28`,`68:31`,`68:32`,`68:34`}; negatives {`68:4`,`68:5`,`68:6`,`68:7`,`68:17`,`68:18`,`68:22`,`68:23`,`68:27`,`68:28`,`68:31`} | prev `questFinishEvent:21` → latest | `SOLVED_WITH_ASSIST` (any assist key) → `attempt_number` (= neg count); `EXCESS_ATTEMPTS` (solved self, no assist, neg ≥ 4) → `attempt_number` (= neg + 1) |
| U2P2 | keys | **remove** `18:99`, `18:223`, `18:224` (6 keys remain: `28:179`,`59:179`,`28:182`,`59:182`,`28:183`,`59:183`); green iff count ≤ 1 | latest `20:26`, start = latest `questFinishEvent:21` before it; color script also fences by client `timestamp` (see note) | `EXCESS_NAV_REMINDERS` → `triggering_number` |
| U2P3 | threshold + reason layer | keys identical (33); green iff count **< 6** (Go: ≤ 6); phase split: Tera = start → first `21:1` in window, Aryn = first `18:231` in window → end | latest `22:18`, start = latest `20:33` before it (doc header says Trigger(Start) `20:26`; script and Go use `20:33`) | `EXCESS_NAV_REMINDERS` → `triggering_number`, `tera_count`, `aryn_count` (needs "earliest event by key in window") |
| U2P4 | reason layer | color identical (`74:21` success; bad {`74:16`,`74:17`,`74:20`,`74:22`}); assist {`74:18`,`74:20`,`74:22`,`74:25`}; negatives {`74:4`,`74:5`,`74:6`,`74:9`,`74:10`,`74:15`,`74:16`,`74:17`}; EXCESS gate = any of {`74:16`,`74:17`} (5th attempt), not a count | prev `23:17` → latest | `SOLVED_WITH_ASSIST` → `attempt_number` (= neg count); `EXCESS_ATTEMPTS` → `attempt_number` (= neg + 1) |
| U2P5 | keys + reason layer | POS add `26:140,142,143,146,147,148`, remove `26:171`; NEG add `26:137,144,145`, remove `26:190`; score = pos − neg/3, green ≥ 4 (unchanged); buckets claim/reasoning/evidence (9 keys each) | prev `23:42` → latest | `EXCESS_MISCLASSIFICATIONS` → `wrong_number`, `claim_wrong`, `reasoning_wrong`, `evidence_wrong` |
| U2P6 | reason layer | color identical (`20:43` pass; `20:44`,`20:45` wrong) | latest `23:42` / latest `20:46`, end must follow start (strict) | `WRONG_EVIDENCE_SELECTED` → `wrong_choice` ("waterfall height" / "salinity" / "waterfall height and salinity") |
| U2P7 | keys + reason layer | **remove** dead `27:19`, `27:21`–`27:24` (15 negatives remain); green iff `27:7` and neg ≤ 3 (unchanged); buckets claim {`27:11`,`27:12`}, both {`27:13`–`27:18`}, irrelevant evidence {`27:20`,`27:25`–`27:30`} | prev `questFinishEvent:54` → latest | `EXCESS_ATTEMPTS` → `attempt_number` (= neg + 1 if success else neg), `wrong_claim_number`, `both_wrong_number`, `irrelevant_evidence_number` |

Unit 2 notes: U2P2/U2P3 color scripts additionally fence counts by the client `timestamp` string (`$gte start.timestamp, $lte end.timestamp`) and treat a missing anchor timestamp as yellow; the U2P3 reason script drops that fence, so the doc is internally inconsistent. The `_id` fence alone gives the same counts unless events were uploaded out of order. Spec states with a yellow cell but no reason code: U2P1/U2P4 with no success and no assist; U2P6 with no pass node and neither wrong node; any point whose window is missing.

### Unit 3
| Point | Verdict | What changes in Go | Window (per script) | Reason codes → variables |
|---|---|---|---|---|
| U3P1 | reason layer | color identical (green iff count `10:30` > 1); new count of {`10:31`,`10:32`} | prev `DialogueNodeEvent:11:22` → latest | `EXCESS_WRONG_RIVERS` → `wrong_river_number` |
| U3P2 | reason layer + start key | color identical (5 − cappedPenalty(c27) − cappedPenalty(c29+c230), green ≥ 3); start key `questFinishEvent:17` fires *after* the end (bogus active attempt) | prev `DialogueNodeEvent:11:34` → latest | `EXCESS_SENSOR_REMINDERS` → `downstream_reminder_number` (11:27), `redundant_reminder_number` (11:29 + 11:230) |
| U3P3 | rewrite (flag branch) | color identical (18 keys incl. success `84:36`, base score + backing-info bonus, green ≥ 3); replace two codes with one; wrong count = 16 keys (excl. `84:36`, `84:38`); buckets claim(5)/reasoning(8)/evidence(3) | prev `questFinishEvent:18` → latest | `EXCESS_ATTEMPTS` → `wrong_argument_number`, `claim_wrong_number`, `reasoning_wrong_number`, `evidence_wrong_number`, `backing_info_phrase` ("opened"/"did not open") |
| U3P4 | rewrite (**production defect**) | start key `questFinishEvent:18` → `questActiveEvent:18` (today's window contains none of the `78:*` puzzle events, so every student is flagged `MISSING_SUCCESS_NODE`); color otherwise identical (gate `78:24`, 8 target keys, score 0 at ≥ 3) | latest `questActiveEvent:18` / latest `73:200`, end must follow start (strict) | `SOLVED_WITH_ASSIST` (assist `78:23` or no gate) → `attempt_number` (7-key count); `EXCESS_ATTEMPTS` → `attempt_number` (8-key count + 1) |
| U3P5 | threshold | green iff `pos − 0.5·neg ≥ 2.5` (Go uses ≥ 3.0; doc marks 2.5 vs 3 "for reconciliation", production script and message say 2.5) | prev `DialogueNodeEvent:10:194` → latest | `EXCESS_WRONG_PLANTINGS` → `wrong_planting_number` |

### Unit 4
| Point | Verdict | What changes in Go | Window (per script) | Reason codes → variables |
|---|---|---|---|---|
| U4P1 | new mechanism (**production defect**) | end trigger becomes the Unit-4 `Soil Key Puzzle` event with `data."Soil Key Puzzle Status"="Finished"` and `data.Unit` ~ `^Unit 4` (eventType + data, no eventKey on these events); Go's `questActiveEvent:39` fires *before* the answer and the puzzle, so the rule can never award the 0.5 + duration points; Started lookup needs the same Unit filter; duration from earliest Unit-4 Started (serverTimestamp) to the Finished anchor; bands unchanged (≤ 30 s → 1, ≤ 90 s → 0.5), `88:5` → 0.5, green ≥ 1 | latest `88:0` / latest Unit-4 Finished, end must follow start (strict) | `SCORE_BELOW_THRESHOLD` → `choice_phrase` ("answered correctly" / "chose 'it's any water found underground' instead of the correct answer on"), `duration_phrase` ("took N seconds to solve" with JS rounding / "has no measured completion time for") |
| U4P2 | new mechanism | start anchor = latest Unit-4 soil-key Finished before the trigger (same eventType + data match); Go's `questActiveEvent:39` re-fires after `88:11` on some runs → false yellow; color otherwise identical (`88:11` success; yellow {`102:9`,`102:10`,`102:12`,`102:18`,`102:23`}); assist {`102:20`,`102:21`,`102:23`}; negatives {`102:3`,`102:4`,`102:7`,`102:9`,`102:10`,`102:12`,`102:18`} | latest `questActiveEvent:48`, start = soil-key Finished before it (zero if none) | `SOLVED_WITH_ASSIST` → `attempt_number` (= neg count); `EXCESS_ATTEMPTS` (no assist, any of `102:9/10/12/18`) → `attempt_number` (= neg + 1) |
| U4P3 | no change (rename metrics) | color identical (soilMachine machine "1" floor "3"/"4" counts, score > 1) | prev `questActiveEvent:50` → latest | `SCORE_BELOW_THRESHOLD` → `floor3_attempts`, `floor4_attempts` |
| U4P4 | operators | machine-1 point requires top = 1 **and bottom = 1** (Go: bottom = 0); depth points use success count **> 0** (Go: == 1); keys unchanged; green > 2 | latest `questActiveEvent:50` / latest `questActiveEvent:36`, end after start (strict) | `SCORE_BELOW_THRESHOLD` → `machine_attempt_number` (top + bottom + machine 2), `wrong_choice_number` (count of {`107:2`,`107:3`,`107:4`,`107:6`}). Doc flags `107:4` in the success list for review |
| U4P5 | threshold + reason layer | yellow iff negatives **≥ 3** (Go: ≥ 4) or no success ({`90:50`,`90:57`}); 13 negative keys unchanged; buckets claim {`90:37`,`90:55`}, reasoning {`90:25`,`90:56`,`90:52`,`90:60`,`90:54`,`90:61`}, evidence {`90:39`,`90:58`,`90:45`,`90:59`,`90:47`} | prev `questActiveEvent:41` → latest | `EXCESS_ATTEMPTS` → `attempt_number` (= neg + 1 if success else neg), `claim_wrong_number`, `reasoning_wrong_number`, `evidence_wrong_number` |
| U4P6 | rewrite | box-id score unchanged ("0" Gravel, "1" Sand, "2" Clay, green ≥ 2) **plus** dialogue fallback: count `92:61` after the latest `92:33` in the window, capped at 3; final = max(box score, dialogue score) | latest `questActiveEvent:41` / latest `questFinishEvent:56` (non-strict) | `WRONG_SOIL_SELECTED` → `wrong_box_number`, `wrong_box_summary` ("the first box (chose Clay, needs Gravel) and …", built from the box-id check) |

### Unit 5
| Point | Verdict | What changes in Go | Window (per script) | Reason codes → variables |
|---|---|---|---|---|
| U5P1 | rewrite | pass needs `100:44` **and zero** of {`100:38`,`100:39`,`100:43`} (Go allowed ≤2); assist keys {`100:40`,`100:41`,`100:43`,`100:46`}; negatives for counting {`100:33`–`100:39`} (7 keys) | latest `questActiveEvent:43` / latest `questFinishEvent:43`, end must follow start (non-strict) | `SOLVED_WITH_ASSIST` → `attempt_number` (= neg count); `EXCESS_ATTEMPTS` → `attempt_number` (= neg count + 1); mutually exclusive on assist |
| U5P2 | no change (rename metrics) | color identical; expose `floor3_attempts`/`floor4_attempts` | latest `questFinishEvent:43` / latest `DialogueNodeEvent:96:1` (non-strict) | `SCORE_BELOW_THRESHOLD` → `floor3_attempts`, `floor4_attempts`. Pending: DualChamber_* machineTypes not counted (flagged for review) |
| U5P3 | keys | add `108:63,64,65,66,68,69` (39 keys total); yellow iff count ≥ 4; buckets claim(12)/reasoning(18)/evidence(9) | latest `DialogueNodeEvent:96:1` / latest `questFinishEvent:44` (non-strict) | `EXCESS_ATTEMPTS` → `wrong_argument_number`, `claim_wrong_number`, `reasoning_wrong_number`, `evidence_wrong_number` |
| U5P4 | reason layer only | color identical (`106:35` success, 11 negatives, zero tolerance) | latest `questFinishEvent:45`, start = latest `questFinishEvent:44` before it (Go's current algorithm) | `WRONG_SETTINGS_SELECTED` → `wrong_run_number`, `failure_phrase` (fixed sentences per sunlight/glass/roof bucket, "(N runs)" suffix, joined with "; and ") |

Unit 5 notes: `questFinishEvent:45` logs twice back-to-back (duplicate grade rows today); U5P1 script header comment still says `questActiveEvent:39`; the new U5P4 doc does not expose the three solar-still selections (EA brief Note 3), but the logs carry `SolarStillDesignEvent` with `data.designSelections.*` if EA scoring is pursued later.

### Reason-code vocabulary after the change

`SOLVED_WITH_ASSIST`, `EXCESS_ATTEMPTS`, `EXCESS_NAV_REMINDERS`, `EXCESS_MISCLASSIFICATIONS`,
`WRONG_EVIDENCE_SELECTED`, `EXCESS_WRONG_RIVERS`, `EXCESS_SENSOR_REMINDERS`,
`EXCESS_WRONG_PLANTINGS`, `SCORE_BELOW_THRESHOLD`, `WRONG_SOIL_SELECTED`,
`WRONG_SETTINGS_SELECTED`, `WRONG_ARG_SELECTED` (U1P3 only). Retired:
`MISSING_SUCCESS_NODE`, `TOO_MANY_NEGATIVES`, `BAD_FEEDBACK`, `HIT_YELLOW_NODE`,
`NO_TRIGGER`, and the never-used `TOO_MANY_TARGETS`, `TOO_MANY_TESTS`,
`HINT_OR_TOO_MANY_GUESSES`, `PUZZLE_TOO_SLOW`. The same code name carries different
variables on different points (`EXCESS_ATTEMPTS` on U2P1/U2P4/U2P7/U3P3/U3P4/U4P2/U4P5/U5P3),
so templates are keyed by (point, code).

**Spec states with a yellow cell but no reason code** (the scripts return nothing):
U2P1/U2P4/U4P2 when the success node is absent and no assist fired; U2P6 when neither
wrong-option node fired; U5P1 when only `100:44` is missing; and every point whose
window is missing or invalid. The grader will store the yellow with an empty `reasons`
list and a `window` note in `metrics`; the dashboard shows a neutral "no details
recorded" line for those.

## 6. Work plan

### Phase A — mhsgrader (this cycle)

| # | Step | Verifies |
|---|------|----------|
| A1 | **Fixture replay harness** (`internal/app/grader/replay_test.go` + `internal/testutil`): import a dump into a scratch DB, run one engine pass, read back grades; loaders for `fixtures.yaml` / `expectations.yaml`. Run it against the unchanged code first and check in the baseline table from §1 as the starting point. | The harness itself; the baseline numbers |
| A2 | **Engine changes**: trigger-to-trigger window in the evaluator (D1); eventType+data trigger form in registry + scanner + index (D5); `reasons` on `Grade` (D3); `LogDataHelper` additions the rules need (counts by key bucket, first/last event by data filter in a window, event-pair duration, phase split by marker keys, client-timestamp windows); config int fix (D8). | Existing points still grade identically on the clean fixture |
| A3 | **Rules, one unit per commit** (U1 → U5), each rule rewritten from its doc's Production Script + Corresponding Scripts, bumped to `_v3`, with the reason variables named exactly as the templates. Order by risk: U1 (1 point), U2 (7, most vocabulary change), U3, U4 (new trigger form, U4P6 fallback), U5. | Replay harness reaches 26/26 colors + codes + variables on all five fixtures; per-rule edge tests |
| A4 | **Reason-code JSON generator + placeholder test** (D4). | Every template placeholder is emitted |
| A5 | **Docs**: CLAUDE.md, ai/context.md, `docs/statistics_and_data_collection.md` (new fields, new vocabulary), a short `docs/updates/grading-sync-092026.md` recording what changed and why. | — |
| A6 | **Deploy + regrade**: `aws_update.sh`, then `aws_reset.sh`, restart, watch the replay complete; spot-check a few students on the dashboard against the Debug timeline. | Production shows the new codes |

Commit and push after each step (Dale's rule); A6 needs Dale's go-ahead on timing.

### Phase B — StrataHub MHS Dashboard (after A6)

| # | Step |
|---|------|
| B1 | Read `reasons` + `variables`; render the point's Instructor Message(s) and Teacher Guidance in the review modal from `mhs_reason_codes.json`; drop the generic "Suggested Actions"; keep a fallback line for legacy codes until the regrade has finished. |
| B2 | Regenerate `mhs_grading_rules.json` (start/trigger/evaluated keys per point drive the Debug timeline annotations) from the same rule definitions so the timeline highlights the keys the v3 rules actually use. |
| B3 | "Started again" display rule for active-after-final (D2b); analytics tab reads the new `metrics` names where they changed. |
| B4 | AI summary inside the pop-up: a per-point "Explain this" button that sends the grade's variables plus that point's context (rubric, curriculum goal, strategies, dialogue excerpt from `feedback-message-for-each-pp/`) to the existing Claude summary path, in the 80–150-word format the examples use. Scope to confirm — see Q5. |
| B5 | Teacher guide: a short Progress-view section on reading the pop-up (the guide currently covers the Devices view only). |

## 7. Questions / decisions needed from Dale

1. **Window semantics (D1).** OK to make each rule compute the exact window its
   validated production script uses (mostly previous-END → END; latest-start/latest-end
   for U2P6, U3P4, U5P1–P3; timestamp-fenced for U2P2/U2P3), and keep start keys only
   for the pencil state and durations? The alternative is to keep today's start-key
   window everywhere and accept replay differences from the validated scripts.
2. **Production regrade (D6).** OK to wipe `mhsgrader.progress_point_grades` and replay
   all of stratalog after the v3 deploy? When (quiet hour, and should teachers be told
   the dashboard will refill over a few minutes)?
3. **Spec items the docs themselves mark "for review".** Implement the production
   scripts exactly as validated and carry these as open items with the grading team,
   rather than deciding them in code: U3P3 counts the success node `84:36` in the color
   sum; U3P5 green at ≥ 2.5 (production/message) vs ≥ 3 (rule table); U4P4 lists `107:4`
   as a success key; U5P2 ignores `DualChamber_*` machine interactions. Agree?
4. **Start keys that fire after their end.** U3P2 (`questFinishEvent:17`) and, until
   fixed, U3P4 (`questFinishEvent:18`). For U3P2, use `DialogueNodeEvent:11:22` (the
   U3P1 end, keeping the chain) as the start? Or should I raise it with the grading team
   first since it is their doc?
5. **Per-point AI pop-up summary (B4).** In scope for the dashboard phase, or a later
   cycle? It needs the per-point context files from `feedback-message-for-each-pp/`
   embedded in stratahub and a Claude call per click.
6. **EA checkpoint scores** (`docs/updates/ea-scores.md`). Leave out of this cycle
   (the new metrics keep it derivable later), or fold it in now?
7. **mhsgrading repo edits.** The docs have internal inconsistencies I will hit while
   implementing (window prose vs script, U2P3 header start key, U5P1 stale comment,
   U5P4 trigger row). Should I edit that repo, or hand a list to its owner?

## 8. References

- Spec: `mhsgrading/grading-logic/mhs-unitX-pointY-grading.md` (Production Script = colour; `## Reason Codes` = codes, messages, Corresponding Scripts; Teacher Guidance)
- Executable transcriptions and fixtures: `mhsgrading/rubric-validation/` (`test_uXpY.py`, `config/fixtures.yaml`), `mhsgrading/reason-code-validation/` (`rc_uXpY.py`, `config/expectations.yaml`); dumps under `mhsgrading/playthrough-logs-and-results/<build>/`
- Latest audit: `mhsgrading/grading-readiness-audit/reports/09-14-26-3/audit-summary.md`
- Go grader: `mhsgrader/internal/app/{grader,rules,store}`; deploy scripts `mhsgrader_update/aws_update.sh`, `aws_reset.sh`
- Dashboard consumer: `stratahub/internal/app/features/mhsdashboard/dashboard.go` (`ProgressGradeItem`, `reasonCodeToMessage`, `buildProgressRows`), review modal in `templates/mhsdashboard_view.gohtml`, key annotations from `stratahub/internal/app/resources/mhs_grading_rules.json`
- Related brief, out of scope here: `mhsgrader/docs/updates/ea-scores.md`
