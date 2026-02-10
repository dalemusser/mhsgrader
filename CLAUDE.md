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

### Rule Interface
```go
type Rule interface {
    ID() string              // e.g., "u2p3_v1"
    Unit() int               // e.g., 2
    Point() int              // e.g., 3
    TriggerKeys() []string   // eventKeys that trigger evaluation
    Evaluate(ctx, db, game, playerId) (Result, error)
}
```

### Result Types
- `Green()` - Success, player demonstrated competency
- `Yellow(reasonCode, metrics)` - Needs improvement, with reason

### Adding a New Rule
1. Create new file in `internal/app/rules/` (e.g., `u3p1.go`)
2. Implement the Rule interface using BaseRule
3. Register in `registry.go`'s `DefaultRegistry()`

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
| `MHSGRADER_REPROCESS_ALL` | false | Reset cursor and reprocess all logs |

## Common Commands

```bash
make build        # Build binary
make build-linux  # Build for Linux (production)
make run          # Run locally
make run-backfill # Run in backfill mode
make tidy         # Sync dependencies
make test         # Run tests
```

## Data Structures

### mhsgrader.progress_point_grades (per-player aggregate)
```js
{
  game: "mhs",
  playerId: "student@mhs.mhs",
  grades: {
    "u1p1": { color: "green", computedAt: ISODate(...), ruleId: "u1p1_v1" },
    "u2p3": { color: "yellow", computedAt: ISODate(...), ruleId: "u2p3_v1",
              reasonCode: "TOO_MANY_TARGETS", metrics: { countTargets: 9, threshold: 6 } }
  },
  lastUpdated: ISODate(...)
}
```

### mhsgrader.grader_state (cursor tracking)
```js
{
  _id: "mhs-grader",
  lastSeenId: ObjectId("..."),
  updatedAt: ISODate(...)
}
```
