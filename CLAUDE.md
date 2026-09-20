# CLAUDE.md - MHSGrader Project Instructions

## Overview

MHSGrader is a standalone background grading service that:
1. Scans `stratalog.logdata` for trigger events (read-only)
2. Evaluates grading rules to determine green/yellow progress point grades
3. Stores grades in `mhsgrader.progress_point_grades` (per-player aggregate documents)
4. Runs on its own EC2 instance, sharing DocumentDB cluster with stratahub and stratalog

## Database Architecture

Uses two databases on the same DocumentDB cluster:
- **stratalog** (read-only): Source of log data (`logdata` collection)
- **mhsgrader** (read/write): Stores grades and state (`progress_point_grades`, `grader_state`)

## Architecture

```
mhsgrader/
├── cmd/mhsgrader/main.go      # Entry point (no HTTP server)
├── internal/app/
│   ├── bootstrap/             # Configuration and DB connection
│   ├── grader/                # Main engine loop
│   │   ├── engine.go          # Poll-evaluate-store loop
│   │   ├── scanner.go         # _id cursor scanning
│   │   └── evaluator.go       # Rule dispatch and grade storage
│   ├── rules/                 # Grading rules
│   │   ├── rule.go            # Rule interface
│   │   ├── registry.go        # eventKey -> Rule mapping
│   │   └── u1p1.go ... u2p7.go  # Individual rules
│   └── store/                 # Data access
│       ├── graderstate/       # Cursor persistence
│       ├── progressgrades/    # Grade storage
│       └── logdata/           # Log queries (read-only)
├── config.example.toml
├── Makefile
└── go.mod
```

## Key Patterns

### Rules follow the grading specification
The specification is `mhsgrading/grading-logic/mhs-unitN-pointM-grading.md`
(Production Script = colour; `## Reason Codes` = which code fires and the
message variables). Its Python transcriptions in `mhsgrading/rubric-validation/`
and `mhsgrading/reason-code-validation/` are validated against five playthrough
fixtures; the Go rules transcribe them. **Read `docs/rule-authoring.md` before
touching a rule** (window kinds, event matchers, helpers, Result/Reason, the
replay harness). Each rule declares the window its script uses
(`WithWindow(WindowPrevTrigger)` or the default start-before-end, optionally
`WithTimestampFence()`); anchors that are not eventKeys (the Unit 4 soil-key
puzzle close) are `EventMatch`es.

### Result Types
- `PassedWithMetrics(metrics)` — green
- `FlaggedWith(metrics, Reason{Code, Variables}...)` — yellow with the spec's
  reason code(s); `Variables` are the Instructor Message placeholders verbatim
- `Flagged(code, metrics)` — yellow with a code and no variables (legacy form)

### Verify every change with the replay harness
```bash
go test ./internal/app/grader/ -run TestFixtureReplay -v   # needs local MongoDB + ../mhsgrading
```
It replays the five fixtures through the real scanner + evaluator and checks
colours, reason codes and variables per point (26/26 on every fixture is the
bar). `MHSGRADER_TEST_UNITS=2` restricts the assertions; `MHSGRADING_DIR`
points at the mhsgrading checkout.

### Adding a New Rule
1. Create new file in `internal/app/rules/` (e.g., `u3p1.go`) per `docs/rule-authoring.md`
2. Register in `registry.go`'s `DefaultRegistry()`
3. Run the replay harness

## Configuration

Environment variables use `MHSGRADER_` prefix:

| Variable | Default | Description |
|----------|---------|-------------|
| `MHSGRADER_MONGO_URI` | mongodb://localhost:27017 | MongoDB connection |
| `MHSGRADER_LOG_DATABASE` | stratalog | Database for reading logs |
| `MHSGRADER_GRADES_DATABASE` | mhsgrader | Database for storing grades |
| `MHSGRADER_GAME` | mhs | Game identifier |
| `MHSGRADER_SCAN_INTERVAL` | 5s | Poll interval |
| `MHSGRADER_BATCH_SIZE` | 500 | Max logs per scan |

## Common Commands

```bash
make build        # Build binary
make build-linux  # Build for Linux (production)
make run          # Run locally
make run-reset    # Run with --reset to clear all state and grades
make tidy         # Sync dependencies
make test         # Run tests

# Or directly with the binary:
./mhsgrader           # Normal operation (continues from cursor)
./mhsgrader --reset   # Clear all state and grades, then exit
./mhsgrader --once    # Grade everything pending, then exit
go run ./cmd/mhsreasoncodes -o ../stratahub/internal/app/resources/mhs_reason_codes.json  # regenerate the dashboard's message templates from the spec
```

## Data Structures

### mhsgrader.progress_point_grades (per-player aggregate; one array of attempts per point)
```js
{
  game: "mhs",
  user_id: "665f1a2b3c4d5e6f7a8b9c0d",      // hex of stratahub.users._id
  currentUnit: "unit3",
  grades: {
    "u1p1": [ { attempt: 1, status: "passed", ruleId: "u1p1_v2", computedAt: ISODate(...),
                metrics: { mistakeCount: 0 }, startTime, endTime, durationSecs, activeDurationSecs } ],
    "u2p1": [ { attempt: 1, status: "flagged", ruleId: "u2p1_v3",
                reasonCode: "SOLVED_WITH_ASSIST",                       // first code (legacy readers)
                reasons: [ { code: "SOLVED_WITH_ASSIST", variables: { attempt_number: 4 } } ],
                metrics: { mistakeCount: 4, ... } } ]
  },
  lastUpdated: ISODate(...)
}
```
`status` is `active` (started), `passed` (green) or `flagged` (yellow). A flagged
grade may carry no `reasons` when the spec defines no code for that state.

### mhsgrader.grader_state (cursor tracking)
```js
{
  _id: "mhs-grader",
  lastSeenId: ObjectId("..."),
  updatedAt: ISODate(...)
}
```
