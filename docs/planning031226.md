# MHS Grader — Planning 2026-03-12

> **Updated** after discussion — reflects decisions on unified implementation, status terminology, and start+end windows.

## Decisions

1. **Unified implementation across Units 1–5** — All units will use the same patterns and design. Existing U1–3 rules will be rewritten to match the mhsgrading/ documentation. The mhsgrading/ directory is the single source of truth.
2. **mhsgrader is the single data source for the dashboard** — All progress states (active, passed, flagged) are stored in `progress_point_grades`. The dashboard does not query stratalog directly.
3. **Start+end trigger windows for all units** — Every progress point uses explicit `Trigger(Start)` and `Trigger(End)` events from the grading docs. The scanner and evaluator will be updated to support this pattern.
4. **Status terminology** — Replace "green"/"yellow" with meaningful terms:
   - `pending` (or no record) — not started
   - `active` — student has triggered the start event but not the end event
   - `passed` — completed successfully (was "green")
   - `flagged` — completed with performance concerns needing review (was "yellow")
5. **Unit-level "in unit" tracking** — mhsgrader detects unit start events (from `mhs-unit-start.md`) and stores which unit a student is currently in.
6. **Reason codes** — Implement for Units 1–3 now (specs available). Units 4–5 reason codes are being developed by the analytics team; allow for future implementation.

## Architecture

- **Engine** (`grader/engine.go`): Polls every 5s, calls scanner then evaluator
- **Scanner** (`grader/scanner.go`): Cursor-based scan of `stratalog.logdata` using `_id` ordering
- **Evaluator** (`grader/evaluator.go`): Dispatches trigger events to registered rules, stores grades
- **Registry** (`rules/registry.go`): Maps trigger event keys to rules; `DefaultRegistry()` registers all rules
- **Rules** (`rules/u*p*.go`): Individual rule files implementing the `Rule` interface
- **Helpers** (`rules/helpers.go`): `LogDataHelper` with attempt windowing, event counting, data field checks

### Changes Needed

**Scanner/Evaluator**: Must support start+end trigger pattern. When a start trigger is seen, store `active` status. When end trigger fires, evaluate the rule using the window between start and end `_id` values, then store `passed` or `flagged`.

**Status field**: Change from `status: "green"/"yellow"` to `status: "active"/"passed"/"flagged"` in `progress_point_grades` documents.

**Unit-level tracking**: Detect unit start events and store current unit for each student (separate from progress point grades).

## Units — All 26 Progress Points

### Unit 1: Water Underground — 4 points

| Point | Name | Start Trigger | End Trigger |
|-------|------|--------------|-------------|
| u1p1 | Beneath the Surface | `questActiveEvent:28` | Per grading doc |
| u1p2 | Where Does Water Go? | `DialogueNodeEvent:31:29` | Per grading doc |
| u1p3 | Gravity of the Situation | `DialogueNodeEvent:30:98` | Per grading doc |
| u1p4 | Deep Impact | `questActiveEvent:34` | Per grading doc |

### Unit 2: Water Flows — 7 points

| Point | Name | Start Trigger | End Trigger |
|-------|------|--------------|-------------|
| u2p1 | Water Cycle Basics | `DialogueNodeEvent:18:1` | Per grading doc |
| u2p2 | Puddle Puzzler | `questFinishEvent:21` | Per grading doc |
| u2p3 | Runoff Ruckus | `DialogueNodeEvent:20:26` | Per grading doc |
| u2p4 | Stream Scene | `DialogueNodeEvent:22:18` | Per grading doc |
| u2p5 | Erosion Equation | `DialogueNodeEvent:23:17` | Per grading doc |
| u2p6 | Watershed Moment | `DialogueNodeEvent:23:42` | Per grading doc |
| u2p7 | Flow Control | `DialogueNodeEvent:20:46` | Per grading doc |

### Unit 3: Water Quality — 5 points

| Point | Name | Start Trigger | End Trigger |
|-------|------|--------------|-------------|
| u3p1 | Testing the Waters | `DialogueNodeEvent:10:1` | Per grading doc |
| u3p2 | Source Searching | `questFinishEvent:17` | Per grading doc |
| u3p3 | Filter Frenzy | `DialogueNodeEvent:11:34` | Per grading doc |
| u3p4 | Clean Sweep | `questFinishEvent:18` | Per grading doc |
| u3p5 | Water Report | `DialogueNodeEvent:73:200` | Per grading doc |

### Unit 4: Dig Deeper (Groundwater) — 6 points

| Point | Name | Start Trigger | End Trigger | New Patterns |
|-------|------|--------------|-------------|--------------|
| u4p1 | Well What Have We Here? | Per grading doc | Per grading doc | Time-based scoring (duration between events) |
| u4p2 | Power Play (Floors 1-2) | Per grading doc | Per grading doc | Gate pattern |
| u4p3 | Power Play (Floors 3-4) | Per grading doc | Per grading doc | Multi-component + `data` field reads (soilMachine) |
| u4p4 | Power Play (Floor 5) | Per grading doc | Per grading doc | Multi-component + `data` field reads |
| u4p5 | Saving Cadet Anderson | Per grading doc | Per grading doc | Gate + count |
| u4p6 | Desert Delicacies | Per grading doc | Per grading doc | Latest-event-per-category + `data` field reads (TerasGardenBox) |

### Unit 5: Rise and Return (Atmospheric Water) — 4 points

| Point | Name | Start Trigger | End Trigger | New Patterns |
|-------|------|--------------|-------------|--------------|
| u5p1 | If I Had a Nickel (Floors 1-2) | Per grading doc | Per grading doc | Gate + count |
| u5p2 | If I Had a Nickel (Floors 3-4) | Per grading doc | Per grading doc | Attempt counting + `data` field reads (WaterChamberEvent) |
| u5p3 | What Happened Here? | Per grading doc | Per grading doc | Windowed count |
| u5p4 | Water Problems Require Water Solutions | Per grading doc | Per grading doc | Gate + count (threshold=0) |

## Unit Start Events

From `mhs-unit-start.md`:

| Unit | Start Event Key |
|------|----------------|
| 1 | `questActiveEvent:28` |
| 2 | `DialogueNodeEvent:18:1` |
| 3 | `DialogueNodeEvent:10:1` |
| 4 | `DialogueNodeEvent:88:0` |
| 5 | `questActiveEvent:43` |

## Helper Methods Needed

1. **Time duration between events**: u4p1 needs elapsed time between two events. `DurationBetweenEvents()` helper.
2. **Data field queries**: u4p3, u4p4, u4p6, u5p2 need to read `data` fields from log entries. Extend beyond existing `HasEventTypeWithDataInWindow()`.
3. **Latest event per category**: u4p6 needs most recent event per box/category. `LatestEventPerCategory()` helper.
4. **Start+end window helpers**: All rules need window bounded by start `_id` to end `_id`.

## Reason Codes

- **Units 1–3**: Defined in `Reason-codes-and-instructor-messages.md`. Implement now.
- **Units 4–5**: Being developed by analytics team. Not yet available. Design rules to accept reason codes when they become available.

## Implementation Sequence

1. **Update `mhs_progress_points.json`** — Fix point counts and names for U4-U5 (no code dependencies)
2. **Refactor status terminology** — Change "green"/"yellow" to "passed"/"flagged" throughout grader codebase
3. **Add start+end window infrastructure** — Scanner and evaluator changes to support start/end triggers and `active` status
4. **Rewrite U1–U3 rules** — Using start+end windows and new status terminology, matching mhsgrading/ docs
5. **Implement U4–U5 rules** — Simple patterns first (u4p2, u4p5, u5p1, u5p3, u5p4), then complex (u4p1, u4p3, u4p4, u4p6, u5p2)
6. **Add unit-level tracking** — Detect unit start events, store current unit per student
7. **Implement reason codes for U1–U3**
8. **Add reason codes for U4–U5** when available

## Testing

- Use `--reset` flag to clear state and regrade from scratch
- Verify grades in `mhsgrader.progress_point_grades` collection directly
- Cross-reference with stratalog events for test players
- Verify `active` status appears for students mid-activity
- Verify unit-level "in unit" tracking matches student position
