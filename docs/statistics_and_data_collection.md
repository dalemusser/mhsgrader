# Statistics and Data Collection

This document describes the data the mhsgrader collects, how it collects it, and why. It covers the full pipeline from raw game telemetry through grading rules to the stored grade records used by the StrataHub dashboard.

---

## Overview

The mhsgrader is a continuous grading engine for Mission HydroSci (MHS), an educational game that teaches hydroscience concepts across 5 units with 26 progress points. The grader:

1. **Scans** the `stratalog` database for game telemetry events matching registered trigger keys
2. **Evaluates** each event against grading rules that measure student performance
3. **Stores** detailed grade records in the `progress_point_grades` collection in the mhsgrader database
4. **Serves** this data to the StrataHub dashboard for teacher/leader review

---

## Data Sources

### Input: Game Telemetry (`stratalog` Database)

The grader reads from a `log_entries` collection in the stratalog database. Each entry represents a discrete game event.

**LogEntry structure:**

| Field | Type | Description |
|-------|------|-------------|
| `_id` | ObjectID | MongoDB document ID (used for ordering and windowing) |
| `game` | string | Game identifier (e.g., `"mhs"`) |
| `playerId` | string | Player login ID (matches `login_id` in StrataHub users) |
| `eventType` | string | Event category (e.g., `"DialogueNodeEvent"`, `"soilMachine"`, `"WaterChamberEvent"`) |
| `eventKey` | string | Unique event identifier (e.g., `"DialogueNodeEvent:23:42"`) — used for grading triggers |
| `timestamp` | time | Client-side timestamp |
| `serverTimestamp` | time | Server-side timestamp (authoritative for ordering and duration calculations) |
| `data` | map | Event-specific payload (e.g., `{"soilType": "Clay", "boxId": "2"}`) |

### Output: Grade Records (`progress_point_grades` Collection)

One document per player per game, containing an array of grades for each progress point.

**PlayerGrades document structure:**

```json
{
  "game": "mhs",
  "playerId": "student_login_id",
  "currentUnit": "unit3",
  "lastUpdated": "2026-03-22T10:15:00Z",
  "grades": {
    "u1p1": [
      {
        "attempt": 1,
        "status": "passed",
        "computedAt": "2026-03-20T14:30:00Z",
        "ruleId": "u1p1_v2",
        "metrics": { "mistakeCount": 0 },
        "startTime": "2026-03-20T14:25:00Z",
        "endTime": "2026-03-20T14:30:00Z",
        "durationSecs": 300.0,
        "activeDurationSecs": 245.5
      }
    ],
    "u2p5": [
      {
        "attempt": 1,
        "status": "flagged",
        "computedAt": "2026-03-21T09:00:00Z",
        "ruleId": "u2p5_v2",
        "reasonCode": "TOO_MANY_NEGATIVES",
        "metrics": { "posCount": 3, "mistakeCount": 8, "score": -0.67 },
        "startTime": "2026-03-21T08:50:00Z",
        "endTime": "2026-03-21T09:00:00Z",
        "durationSecs": 600.0,
        "activeDurationSecs": 480.2
      },
      {
        "attempt": 2,
        "status": "passed",
        "computedAt": "2026-03-22T10:15:00Z",
        "ruleId": "u2p5_v2",
        "metrics": { "posCount": 18, "mistakeCount": 2, "score": 17.33 },
        "startTime": "2026-03-22T10:05:00Z",
        "endTime": "2026-03-22T10:15:00Z",
        "durationSecs": 600.0,
        "activeDurationSecs": 520.0
      }
    ]
  }
}
```

---

## Grade Fields

Every grade record stores the following fields:

| Field | Type | Description |
|-------|------|-------------|
| `attempt` | int | 1-based attempt number for this progress point |
| `status` | string | `"active"` (started, not finished), `"passed"` (success), or `"flagged"` (needs review) |
| `computedAt` | time | When the grade was computed |
| `ruleId` | string | Rule version that produced this grade (e.g., `"u2p5_v2"`) |
| `reasonCode` | string | Why the grade was flagged (omitted for passed/active) |
| `metrics` | map | Rule-specific performance metrics (see per-rule details below) |
| `startTime` | time | When the student started the activity (from the start event's `serverTimestamp`) |
| `endTime` | time | When the student completed the activity (from the end/trigger event's `serverTimestamp`) |
| `durationSecs` | float64 | Wall-clock duration from start to end in seconds |
| `activeDurationSecs` | float64 | Active duration excluding idle gaps longer than the configured threshold |

### Duration Calculations

**Wall-clock duration** (`durationSecs`): `endEvent.serverTimestamp - startEvent.serverTimestamp`

**Active duration** (`activeDurationSecs`): Walks all log entries between start and end events chronologically, summing only the gaps between consecutive entries that are shorter than the `activeGapThreshold` (configurable, typically 2-5 minutes). This excludes idle time, breaks, and periods where the student left the game open without interacting.

---

## Attempt Lifecycle

Each progress point tracks a sequence of attempts as an array:

1. **Start event** fires → `AppendActiveIfNeeded` creates an `"active"` grade entry with the start timestamp and attempt number
2. **End/trigger event** fires → `AppendGrade` replaces the active entry (preserving its attempt number) with the final `"passed"` or `"flagged"` grade, adding metrics, end time, and durations
3. If the student **replays** the activity, a new start event fires → a new `"active"` entry is appended with `attempt = previous_count + 1`
4. The cycle repeats, building a full attempt history

If no active entry exists when the end event fires (e.g., grader wasn't running when the student started), the grade is appended directly as a new entry.

---

## Windowing: Start-to-End Event Scoping

Each rule defines **start keys** (events marking activity beginning) and **trigger keys** (events marking activity completion). The evaluator builds an `EvalContext` containing an `AttemptWindow` that scopes all queries to only the events between the start and end of the current attempt:

- **Window**: `(startEvent._id, endEvent._id]` — exclusive on start, inclusive on end
- This ensures that if a student replays an activity, only the events from the *current* attempt are counted, not events from previous attempts
- If no start event is found, the window starts from the zero ObjectID (unbounded)

---

## Unit Start Events

The grader tracks which unit a student is currently in. These are tracked separately from progress point rules:

| Unit | Start Event Key |
|------|----------------|
| Unit 1 | `questActiveEvent:28` |
| Unit 2 | `DialogueNodeEvent:18:1` |
| Unit 3 | `DialogueNodeEvent:10:1` |
| Unit 4 | `DialogueNodeEvent:88:0` |
| Unit 5 | `questActiveEvent:43` |

---

## Reason Codes

When a grade is flagged, a reason code explains why:

| Code | Dashboard Message |
|------|------------------|
| `NO_TRIGGER` | Student has not yet completed the trigger event for this activity. |
| `TOO_MANY_TARGETS` | Student used more targets than allowed for efficient problem-solving. |
| `TOO_MANY_TESTS` | Student ran more tests than expected, may need guidance on efficiency. |
| `TOO_MANY_NEGATIVES` | Student received too many negative responses during the activity. |
| `MISSING_SUCCESS_NODE` | Student did not reach the expected success outcome. |
| `SCORE_BELOW_THRESHOLD` | Student's score was below the expected threshold. |
| `HINT_OR_TOO_MANY_GUESSES` | Student needed hints or made too many incorrect attempts. |
| `PUZZLE_TOO_SLOW` | Student took longer than expected to complete the puzzle. |
| `WRONG_ARG_SELECTED` | Student needed multiple attempts to construct a correct scientific argument. |
| `BAD_FEEDBACK` | Student received repeated corrective feedback during the activity. |
| `HIT_YELLOW_NODE` | Student made an incorrect selection when evaluating evidence. |

---

## Progress Points and Grading Rules

### Unit 1: Introduction and Argumentation Basics

#### U1P1 — Getting Your Space Legs
- **Activity**: Meet your captain and get to know the game controls
- **Type**: Completion-only
- **Start**: `questActiveEvent:28`
- **Trigger**: `DialogueNodeEvent:31:29`
- **Scoring**: Always passes on completion
- **Metrics**: `mistakeCount` (always 0)

#### U1P2 — Info and Intros
- **Activity**: Meet your fellow cadets and collect argument pieces
- **Type**: Completion-only
- **Start**: `DialogueNodeEvent:31:29`
- **Trigger**: `DialogueNodeEvent:30:98`
- **Scoring**: Always passes on completion
- **Metrics**: `mistakeCount` (always 0)

#### U1P3 — Defend the Expedition
- **Activity**: Learn the argumentation engine and identify claims
- **Type**: Negative event count
- **Start**: `DialogueNodeEvent:30:98`
- **Trigger**: `questActiveEvent:34`
- **Scoring**: Passed if zero wrong-argument selections; Flagged (`WRONG_ARG_SELECTED`) if any
- **Metrics**: `mistakeCount` (count of yellow/wrong nodes hit)

#### U1P4 — What Was That?
- **Activity**: Make contact with ancient aliens and escape the ship
- **Type**: Completion-only
- **Start**: `questActiveEvent:34`
- **Trigger**: `DialogueNodeEvent:33:19`
- **Scoring**: Always passes on completion
- **Metrics**: `mistakeCount` (always 0)

---

### Unit 2: Topographic Maps and Watersheds

#### U2P1 — Escape the Ruin
- **Activity**: Navigate alien ruins and match topographic maps to elevation profiles
- **Type**: Success node + negative count
- **Start**: `DialogueNodeEvent:18:1`
- **Trigger**: `questFinishEvent:21`
- **Scoring**: Passed if success node present AND zero yellow nodes; Flagged if missing success (`MISSING_SUCCESS_NODE`) or too many negatives (`TOO_MANY_NEGATIVES`)
- **Metrics**: `mistakeCount` (count of yellow nodes)

#### U2P2 — Foraged Forging
- **Activity**: Build a hover board and find location by reading a topographic map
- **Type**: Negative event count
- **Start**: `questFinishEvent:21`
- **Trigger**: `DialogueNodeEvent:20:26`
- **Scoring**: Passed if count of bad feedback events <= 1; Flagged (`BAD_FEEDBACK`) if > 1
- **Metrics**: `mistakeCount` (count of bad feedback events)

#### U2P3 — Getting the Band Back Together Part II
- **Activity**: Find crew locations by reading a topographic map
- **Type**: Negative event count with custom windowing
- **Start**: `DialogueNodeEvent:20:33` (the actual activity start within the broader point)
- **Trigger**: `DialogueNodeEvent:22:18`
- **Scoring**: Passed if count of wrong-direction prompts <= 6; Flagged (`BAD_FEEDBACK`) if > 6
- **Metrics**: `mistakeCount` (count of wrong-direction events)

#### U2P4 — Investigate the Temple
- **Activity**: Navigate ruins to find Jasper, relate watershed size to flow rate
- **Type**: Success node + negative count
- **Start**: `DialogueNodeEvent:22:18`
- **Trigger**: `DialogueNodeEvent:23:17`
- **Scoring**: Passed if success node AND zero bad feedback; Flagged if missing success (`MISSING_SUCCESS_NODE`) or negatives (`TOO_MANY_NEGATIVES`)
- **Metrics**: `mistakeCount` (count of bad feedback events)

#### U2P5 — Classified Information
- **Activity**: Fix DANI by identifying parts of a scientific argument
- **Type**: Weighted positive/negative scoring
- **Start**: `DialogueNodeEvent:23:17`
- **Trigger**: `DialogueNodeEvent:23:42`
- **Formula**: `score = posCount - (negCount / 3.0)`; Pass threshold: >= 4.0
- **Scoring**: Passed if score >= 4; Flagged (`TOO_MANY_NEGATIVES`) otherwise
- **Metrics**:
  - `posCount` — number of correct identification events (22 possible positive keys)
  - `mistakeCount` — number of incorrect identification events (25 possible negative keys)
  - `score` — computed weighted score

#### U2P6 — Which Watershed? Part I
- **Activity**: Collect evidence to construct an argument about watershed size
- **Type**: Success node + negative count
- **Start**: `DialogueNodeEvent:23:42`
- **Trigger**: `DialogueNodeEvent:20:46`
- **Scoring**: Passed if pass node AND zero yellow nodes; Flagged if missing (`MISSING_SUCCESS_NODE`) or yellow hit (`HIT_YELLOW_NODE`)
- **Metrics**: `mistakeCount` (count of yellow nodes)

#### U2P7 — Which Watershed? Part II
- **Activity**: Build an argument supporting a claim with evidence
- **Type**: Success node + negative count threshold
- **Start**: `DialogueNodeEvent:20:46`
- **Trigger**: `questFinishEvent:54`
- **Scoring**: Passed if success node AND neg count <= 3; Flagged (`WRONG_ARG_SELECTED`) otherwise
- **Metrics**: `mistakeCount` (count of negative dialogue events, up to 20 possible keys)

---

### Unit 3: Water Flow, Pollution, and Ecosystems

#### U3P1 — Supply Run
- **Activity**: Send crates by identifying water flow direction based on watershed map
- **Type**: Target event count
- **Start**: `DialogueNodeEvent:10:1`
- **Trigger**: `DialogueNodeEvent:11:22`
- **Scoring**: Passed if count of target events > 1; Flagged (`TOO_MANY_NEGATIVES`) if <= 1
- **Metrics**:
  - `count` — raw count of target events (`DialogueNodeEvent:10:30`)
  - `mistakeCount` — `3 - count` when flagged, 0 when passed

#### U3P2 — Pollution Solution Part I
- **Activity**: Find pollutant source by predicting dissolved material spread
- **Type**: Capped penalty scoring
- **Start**: `questFinishEvent:17`
- **Trigger**: `DialogueNodeEvent:11:34`
- **Formula**: `score = 5 - cappedPenalty(c27) - cappedPenalty(c29 + c230)` where `cappedPenalty(n)` = 0 if n<=1, 1 if n<=3, 2 if n>=4. Pass threshold: >= 3
- **Scoring**: Passed if score >= 3; Flagged (`BAD_FEEDBACK`) otherwise
- **Metrics**:
  - `c27` — count of `DialogueNodeEvent:11:27` (first penalty source)
  - `c29` — count of `DialogueNodeEvent:11:29` (second penalty source)
  - `c230` — count of `DialogueNodeEvent:11:230` (third penalty source)
  - `score` — computed capped penalty score
  - `mistakeCount` — total of c27 + c29 + c230

#### U3P3 — Pollution Solution Part II
- **Activity**: Construct an argument about pollutant location with reasoning
- **Type**: Base score + bonus
- **Start**: `DialogueNodeEvent:11:34`
- **Trigger**: `questFinishEvent:18`
- **Formula**: `baseScore` from incorrect argument count (3 if <=3, 2 if 4, 1 if 5, 0 if >=6) + 1 bonus if student used BackingInfoPanel. Pass threshold: total >= 3
- **Scoring**: Passed if totalScore >= 3; Flagged (`MISSING_SUCCESS_NODE` if no backing info, `WRONG_ARG_SELECTED` if backing info was used but still too many mistakes)
- **Metrics**:
  - `baseScore` — score from incorrect argument count alone (0-3)
  - `usedBackingInfo` — boolean, whether the student accessed the BackingInfoPanel tool
  - `totalScore` — baseScore + bonus (0-4)
  - `mistakeCount` — count of incorrect argument target events (18 possible keys)

#### U3P4 — Forsaken Facility
- **Activity**: Navigate ruins and solve puzzles about material movement through waterways
- **Type**: Gate check + count scoring
- **Start**: `questFinishEvent:18`
- **Trigger**: `DialogueNodeEvent:73:200`
- **Scoring**: Must have gate key event; then score by target count: 2 if count==0, 1 if count<=2, 0 if count>=3. Passed if count <= 2; Flagged (`MISSING_SUCCESS_NODE` if no gate, `TOO_MANY_NEGATIVES` if too many targets)
- **Metrics**:
  - `score` — computed score (0, 1, or 2)
  - `mistakeCount` — count of negative target events (8 possible keys)

#### U3P5 — Part of a Balanced Ecosystem
- **Activity**: Help plant seeds by predicting dissolved nutrient spread
- **Type**: Weighted positive/negative scoring
- **Start**: `DialogueNodeEvent:73:200`
- **Trigger**: `DialogueNodeEvent:10:194`
- **Formula**: `score = posCount * 1.0 - negCount * 0.5`. Pass threshold: >= 3.0
- **Scoring**: Passed if score >= 3; Flagged (`TOO_MANY_NEGATIVES`) otherwise
- **Metrics**:
  - `posCount` — count of correct placement events (`DialogueNodeEvent:73:163`)
  - `score` — computed weighted score
  - `mistakeCount` — count of incorrect events (3 possible negative keys)

---

### Unit 4: Soil, Infiltration, and Groundwater

#### U4P1 — Well What Have We Here?
- **Activity**: Search for a water source, demonstrate understanding of water table
- **Type**: Multi-component score with time bonus
- **Start**: `DialogueNodeEvent:88:0`
- **Trigger**: `questActiveEvent:39`
- **Formula**: +0.5 if correct choice (`DialogueNodeEvent:88:5`); +1.0 duration bonus if Soil Key Puzzle completed in <=30s, +0.5 if 30-90s. Pass threshold: >= 1.0
- **Scoring**: Passed if score >= 1; Flagged (`SCORE_BELOW_THRESHOLD`) otherwise
- **Metrics**:
  - `hasCorrectAnswer` — boolean, whether the correct dialogue choice was made
  - `puzzleDurationSecs` — time in seconds to complete the Soil Key Puzzle
  - `durationBonus` — time bonus awarded (0, 0.5, or 1.0)
  - `score` — total computed score
  - `mistakeCount` — 1 if wrong answer, 0 if correct

#### U4P2 — Power Play (Floors 1 & 2)
- **Activity**: Explore ruins, demonstrate infiltration rate / soil type understanding
- **Type**: Success node + negative count
- **Start**: `questActiveEvent:39`
- **Trigger**: `questActiveEvent:48`
- **Scoring**: Passed if success node AND zero yellow feedback; Flagged if missing (`MISSING_SUCCESS_NODE`) or negatives (`TOO_MANY_NEGATIVES`)
- **Metrics**: `mistakeCount` (count of yellow feedback events, 5 possible keys)

#### U4P3 — Power Play (Floors 3 & 4)
- **Activity**: Manipulate alien soil machines to demonstrate infiltration understanding
- **Type**: Per-floor attempt scoring
- **Start**: `questActiveEvent:48`
- **Trigger**: `questActiveEvent:50`
- **Formula**: +1 if floor 3 has exactly 1 `soilMachine` attempt (machine 1); +2 if floor 4 has 1 attempt, +1 if floor 4 has 2 attempts. Pass threshold: > 1
- **Scoring**: Passed if score > 1; Flagged (`SCORE_BELOW_THRESHOLD`) otherwise
- **Metrics**:
  - `floor3Attempts` — soilMachine event count for floor 3, machine 1
  - `floor4Attempts` — soilMachine event count for floor 4, machine 1
  - `score` — computed score
  - `mistakeCount` — total attempts across both floors

#### U4P4 — Power Play (Floor 5) + You Know the Drill
- **Activity**: Soil machine interactions + dialogue about drilling a well
- **Type**: Multi-component scoring
- **Start**: `questActiveEvent:50`
- **Trigger**: `questActiveEvent:36`
- **Formula**: +1 if machine 1 floor 5 TopRow==1 and BottomRow==0; +1 if machine 2 floor 5 count==1; +2 if success dialogue count==1 and neg==0, +1 if success==1 and neg==1. Pass threshold: > 2
- **Scoring**: Passed if score > 2; Flagged (`SCORE_BELOW_THRESHOLD`) otherwise
- **Metrics**:
  - `topRowAttempts` — soilMachine floor 5, machine 1, TopRow count
  - `bottomRowAttempts` — soilMachine floor 5, machine 1, BottomRow count
  - `machine2Attempts` — soilMachine floor 5, machine 2 count
  - `successCount` — count of correct dialogue events
  - `score` — total computed score
  - `mistakeCount` — count of negative dialogue events

#### U4P5 — Saving Cadet Anderson
- **Activity**: Construct a complete argument to convince Anderson
- **Type**: Success node + negative threshold
- **Start**: `questActiveEvent:36`
- **Trigger**: `questActiveEvent:41`
- **Scoring**: Must have positive key; then passed if neg count < 4; Flagged (`MISSING_SUCCESS_NODE` if no positive, `TOO_MANY_NEGATIVES` if >= 4)
- **Metrics**: `mistakeCount` (count of negative dialogue events, 13 possible keys)

#### U4P6 — Desert Delicacies
- **Activity**: Film seedlings in ideal soil by matching soil types to garden boxes
- **Type**: Per-box correctness scoring
- **Start**: `questActiveEvent:41`
- **Trigger**: `questFinishEvent:56`
- **Expected**: Box 0 = Gravel, Box 1 = Sand, Box 2 = Clay
- **Scoring**: +1 per box with correct soil type from latest `TerasGardenBox` placement. Passed if score >= 2 (2 of 3 correct); Flagged (`SCORE_BELOW_THRESHOLD`) otherwise
- **Metrics**:
  - `score` — number of boxes with correct soil (0-3)
  - `box0SoilType` — soil type placed in box 0
  - `box0Correct` — boolean, whether box 0 has the right soil
  - `box1SoilType` — soil type placed in box 1
  - `box1Correct` — boolean, whether box 1 has the right soil
  - `box2SoilType` — soil type placed in box 2
  - `box2Correct` — boolean, whether box 2 has the right soil
  - `mistakeCount` — number of incorrect boxes (3 - score)

---

### Unit 5: Water Cycle, Evaporation, and Condensation

#### U5P1 — If I Had a Nickel (Floors 1 & 2)
- **Activity**: Solve puzzles using knowledge of evaporation rate and the water cycle
- **Type**: Success node + negative threshold
- **Start**: `questActiveEvent:43`
- **Trigger**: `questFinishEvent:43`
- **Scoring**: Passed if success node AND neg count <= 2; Flagged if missing success (`MISSING_SUCCESS_NODE`) or too many negatives (`TOO_MANY_NEGATIVES`)
- **Metrics**: `mistakeCount` (count of negative dialogue events, 3 possible keys)

#### U5P2 — If I Had a Nickel (Floors 3 & 4)
- **Activity**: Solve puzzles using knowledge of evaporation and condensation
- **Type**: Per-floor attempt scoring (WaterChamberEvent)
- **Start**: `questFinishEvent:43`
- **Trigger**: `DialogueNodeEvent:96:1`
- **Formula**: Floor 3: +2 if <=6 attempts, +1 if <11. Floor 4: +2 if <=5 attempts, +1 if <10. Pass threshold: >= 3
- **Scoring**: Passed if score >= 3; Flagged (`SCORE_BELOW_THRESHOLD`) otherwise
- **Metrics**:
  - `floor3Attempts` — WaterChamberEvent count for floor 3 (Condenser + Evaporator)
  - `floor4Attempts` — WaterChamberEvent count for floor 4 (Condenser + Evaporator)
  - `score` — computed score
  - `mistakeCount` — total attempts across both floors

#### U5P3 — What Happened Here?
- **Activity**: Provide a counter argument to a faulty claim about the water cycle
- **Type**: Negative event count
- **Start**: `DialogueNodeEvent:96:1`
- **Trigger**: `questFinishEvent:44`
- **Scoring**: Passed if count of negative dialogues < 4; Flagged (`TOO_MANY_NEGATIVES`) if >= 4
- **Metrics**: `mistakeCount` (count of negative dialogue events, 33 possible keys)

#### U5P4 — Water Problems Require Water Solutions
- **Activity**: Fix a solar still using knowledge of evaporation and condensation
- **Type**: Success node + zero tolerance
- **Start**: `questFinishEvent:44`
- **Trigger**: `questFinishEvent:45`
- **Scoring**: Must have success node; then passed only if zero negative events; Flagged (`MISSING_SUCCESS_NODE` if no success, `TOO_MANY_NEGATIVES` if any negatives)
- **Metrics**: `mistakeCount` (count of negative events, 11 possible keys)

---

## Grading Patterns Summary

The 26 rules use these fundamental patterns:

| Pattern | Rules | Description |
|---------|-------|-------------|
| **Completion-only** | U1P1, U1P2, U1P4 | Always passes when trigger fires |
| **Success + negatives** | U2P1, U2P4, U2P6, U4P2, U4P5, U5P1, U5P4 | Must have success node; checked for negative events |
| **Negative count threshold** | U1P3, U2P2, U2P3, U2P7, U5P3 | Count bad events against a threshold |
| **Weighted scoring** | U2P5, U3P5 | Positive events weighted against negative events |
| **Capped penalty scoring** | U3P2 | Penalty function with diminishing returns |
| **Base score + bonus** | U3P3 | Score from mistakes + bonus for tool usage |
| **Gate + count** | U3P4 | Must pass a gate check, then scored by count |
| **Per-floor attempts** | U4P3, U5P2 | Score based on attempt efficiency per floor |
| **Multi-component** | U4P1, U4P4 | Multiple independent scoring components combined |
| **Per-box correctness** | U4P6 | Score by matching correct values to positions |
| **Target count** | U3P1 | Score based on meeting a minimum event count |

---

## Architecture and Processing

### Engine Loop

The grader runs as a continuous process:

1. **Scanner** queries stratalog for new events matching any registered trigger key, starting from the last-seen cursor position
2. Events are processed in `_id` order (chronological by MongoDB ObjectID)
3. For each event, the **Evaluator** determines if it's a unit start, point start, or point end
4. Grades are stored and the cursor advances
5. The scanner sleeps for the configured `scanInterval` and repeats

### Cursor and State

The grader maintains a cursor (the `_id` of the last processed event) in the `grader_state` collection. This allows:
- Resuming after restarts without reprocessing
- The `--reset` flag to clear the cursor and all grades for a full re-grade

### Event Processing Order

For each scanned event:

1. **Unit start check** — if the event key matches a unit start event, update the player's `currentUnit`
2. **Point start handling** — if the event key matches a rule's `StartKeys`, call `AppendActiveIfNeeded` to create an active grade entry
3. **Point end handling** — if the event key matches a rule's `TriggerKeys`:
   - Build `EvalContext` with the window from start event to end event
   - Evaluate the rule to get passed/flagged status and metrics
   - Calculate wall-clock and active durations
   - Store the grade via `AppendGrade`

### Error Handling

- Each rule is evaluated independently; one failure doesn't block others
- If any rule fails, the cursor does not advance past that event, ensuring retry on next scan
- Already-stored grades are safely handled on retry (AppendGrade replaces active entries idempotently)

---

## Dashboard Consumption

The StrataHub dashboard reads grade data directly from the `progress_point_grades` collection in the mhsgrader database:

- **Grid display**: Shows the latest grade for each progress point (last element of each array)
- **Cell states**: pending (no grade), active (blue), passed (green), flagged (yellow)
- **Flagged details**: Shows human-readable reason from `reasonCode` mapping
- **Duration display**: Formats `durationSecs` and `activeDurationSecs` as `m:ss` or `h:mm:ss`
- **Mistake count**: Extracted from `metrics.mistakeCount` for display
- **Attempt count**: Available from the array length for each point
- **Skipped detection**: Active points with passed points after them, or passed points with no duration data
- **Unit progress**: Computed from grades to show completed/current/future unit status

---

## Reset and Re-grading

Running `mhsgrader --reset` performs a complete cleanup:

1. Deletes the grader state cursor from `grader_state`
2. Deletes all grade documents for the configured game from `progress_point_grades`

After reset, the grader re-processes all events from the beginning, rebuilding all grades with the current rule versions. This is useful after rule changes or bug fixes to ensure consistent grading across all students.

---

## Metrics Collected Per Rule (Quick Reference)

All rules include `mistakeCount`. Additional metrics by rule:

| Rule | Additional Metrics |
|------|--------------------|
| U2P5 | `posCount`, `score` |
| U3P1 | `count` |
| U3P2 | `c27`, `c29`, `c230`, `score` |
| U3P3 | `baseScore`, `usedBackingInfo` (bool), `totalScore` |
| U3P4 | `score` |
| U3P5 | `posCount`, `score` |
| U4P1 | `hasCorrectAnswer` (bool), `puzzleDurationSecs`, `durationBonus`, `score` |
| U4P3 | `floor3Attempts`, `floor4Attempts`, `score` |
| U4P4 | `topRowAttempts`, `bottomRowAttempts`, `machine2Attempts`, `successCount`, `score` |
| U4P6 | `score`, `box{0,1,2}SoilType`, `box{0,1,2}Correct` (bool) |
| U5P2 | `floor3Attempts`, `floor4Attempts`, `score` |

All other rules report only `mistakeCount`.
