# mhsgrader — Context for Claude Code

## What This Repo Does

MHSGrader is a standalone background grading service that continuously evaluates student performance in Mission HydroScience by scanning gameplay logs and storing progress point grades in DocumentDB. It runs on its own EC2 instance, reading immutable game events from the shared DocumentDB cluster's `stratalog` database and writing competency grades to the `mhsgrader` database. The service uses a poll-evaluate-store loop with resumable cursor tracking to handle incremental new log events.

## Technology Stack

- **Language:** Go 1.25
- **Framework:** Waffle (custom Go web framework)
- **Database:** MongoDB / AWS DocumentDB (shared cluster with stratahub and stratalog)
- **Logging:** Zap (structured logging)
- **Deployment:** EC2 instance with systemd (mhsgrader.service)
- **Configuration:** Environment variables + TOML (config.example.toml)

## Folder Structure

```
mhsgrader/
├── cmd/mhsgrader/
│   └── main.go              # Entry point; --reset, --once, engine startup
├── cmd/mhsreasoncodes/      # Writes mhs_reason_codes.json for StrataHub from the spec
├── internal/app/
│   ├── bootstrap/           # Configuration, DB connection, schema setup
│   │   ├── config.go
│   │   ├── appconfig.go
│   │   ├── db.go
│   │   ├── dbdeps.go
│   │   ├── hooks.go
│   │   ├── startup.go
│   │   └── shutdown.go
│   ├── grader/              # Main poll-evaluate-store engine
│   │   ├── engine.go        # Coordinates scanning and evaluation loop
│   │   ├── scanner.go       # Scans stratalog for trigger events
│   │   └── evaluator.go     # Dispatches rules and stores grades
│   ├── rules/               # Grading rules (26: U1P1–U5P4), one file per point
│   │   ├── rule.go          # Rule interface and Result types
│   │   ├── registry.go      # eventKey → Rule mapping + unit start tracking
│   │   ├── u1p1.go–u5p4.go  # Individual rule implementations (see docs/rule-authoring.md)
│   │   └── helpers.go       # LogDataHelper: windowed queries by eventKey / eventType+data
│   ├── fixtures/            # Loads the mhsgrading playthrough fixtures for the replay tests
│   ├── reasoncodes/         # Extracts instructor-message templates from the spec (cmd/mhsreasoncodes)
│   └── store/               # Data access layer
│       ├── graderstate/     # Cursor persistence (resumable scanning)
│       ├── progressgrades/  # Grade storage and retrieval
│       └── logdata/         # Read-only queries against stratalog
├── docs/                    # Curriculum context, planning, statistics docs
├── feedback/                # User feedback and design notes
├── bin/                     # Build artifacts
├── config.example.toml      # Configuration template
├── .env.example             # Environment variable template
├── go.mod / go.sum          # Dependency management
├── Makefile                 # Build, run, test, deploy targets
├── mhsgrader.service        # Systemd service file (EC2)
├── CLAUDE.md                # Project-specific instructions
└── README.md
```

The organizing principle: **Poll-Evaluate-Store pipeline**. Scanning is separate from rule evaluation to allow async processing and clear separation of concerns. Rules are organized by unit/point and map event keys to evaluation logic.

## Code Patterns & Conventions

### Naming Conventions
- **Rules:** `UxPyRule` (e.g., `U2P3Rule`) for struct names; `NewUxPyRule()` for constructors
- **Point IDs:** `u{unit}p{point}` (e.g., `u2p3`) for strings and database keys
- **Rule IDs:** `u{unit}p{point}_{version}` (e.g., `u2p3_v2`) with versioning
- **Event keys:** `{EventType}:{nodeID}:{dialogueID}` (e.g., `DialogueNodeEvent:20:33`)
- **Database prefix:** `MHSGRADER_` for environment variables
- **Grader ID:** `{game}-grader` (e.g., `mhs-grader`) for cursor tracking

### Rule Interface & Implementation
Every rule implements the `Rule` interface:
```go
type Rule interface {
    ID() string                      // e.g., "u2p3_v2"
    Unit() int                       // Unit number
    Point() int                      // Progress point number
    PointID() string                 // "u2p3"
    StartKeys() []string             // Event keys that mark activity start
    TriggerKeys() []string           // Event keys that trigger evaluation
    Evaluate(ctx, db, game, userID, ec EvalContext) (Result, error)
}
```

Use `BaseRule` to avoid boilerplate:
```go
type U2P3Rule struct{ BaseRule }

func NewU2P3Rule() *U2P3Rule {
    return &U2P3Rule{NewBaseRule(2, 3, "v2",
        []string{"DialogueNodeEvent:20:33"},       // start keys
        []string{"DialogueNodeEvent:22:18"},       // trigger keys
    )}
}
```

### Result Types
- `Passed()` / `PassedWithMetrics(map)` — Green (competency demonstrated)
- `FlaggedWith(metrics, Reason{Code, Variables}...)` — Yellow with the spec's reason code(s) and the
  Instructor Message variables (e.g. `SOLVED_WITH_ASSIST` with `attempt_number`); `Flagged(code, metrics)` is the legacy single-code form

### Organization Principles
- **Stateless evaluation:** Rules query the log window, don't mutate state
- **Window-based analysis:** each rule declares the window its production script uses (`WindowPrevTrigger` = previous occurrence of the end trigger → this one; default = latest start anchor before the end; optional client-timestamp fence); `EvalContext.Window` is that interval. A re-fired end trigger with no new start is ignored.
- **Non-key anchors:** `EventMatch` (eventType + data fields, e.g. the Unit 4 soil-key puzzle close) can be a start or end anchor; the scanner and evaluator match them alongside eventKeys
- **Reason codes and variables** follow `mhsgrading/grading-logic` verbatim; `docs/grading-sync-plan-092026.md` records the September 2026 sync and `docs/grading-team-questions-092026.md` the open questions
- **Registry pattern:** `DefaultRegistry()` maps event keys to rules for fast dispatch
- **Unit start tracking:** Registry also tracks unit-level events (e.g., `questActiveEvent:28` for Unit 1)

### Error Handling
- **Graceful degradation:** Evaluation failure stops the current batch but preserves the cursor for retry
- **Structured logging:** Zap logger with context (user_id, eventKey, error)
- **DB operations:** MongoDB driver errors are wrapped with context (`fmt.Errorf`)

### Configuration & Bootstrapping
- **Config loading:** `bootstrap.LoadConfig()` merges environment variables + TOML
- **DB connection:** `bootstrap.ConnectDB()` connects to both `stratalog` (read-only) and `mhsgrader` (read/write)
- **Schema setup:** `bootstrap.EnsureSchema()` creates indexes before scanning
- **Startup hooks:** `bootstrap.Startup()` runs pre-scan initialization (e.g., Prometheus metrics)

## Key Dependencies & Gotchas

### Non-Obvious Constraints
1. **Single-cursor scanning:** The grader maintains a single resumable cursor in `grader_state`. All events are processed in _id order. If evaluation fails, the cursor does not advance, ensuring no logs are skipped.
2. **Read-only stratalog:** The scanner only reads from `stratalog.logdata`. Any log query must use the `LogDataHelper` to avoid cross-database transactions on DocumentDB.
3. **Dual-database architecture:** Both `stratalog` and `mhsgrader` live on the same DocumentDB cluster but as separate logical databases. Ensure connection URIs distinguish them.
4. **EventKey format is critical:** Event keys like `DialogueNodeEvent:20:33` are parsed and used as registry lookups. A typo in a rule's start/trigger keys will silently miss events.
5. **Version suffix on rule IDs:** Rules are versioned (e.g., `v1`, `v2`) to allow updating logic without conflicting with old stored grades. When you change a rule's logic, bump the version in `NewBaseRule()`.

### Common Mistakes
- **Forgetting to register a new rule:** Adding a rule file but not calling `reg.Register(NewUxPyRule())` in `DefaultRegistry()` means it never evaluates.
- **Wrong window handling:** Some rules require a start event to establish a time window. If the start key is missing, `ec.Window` will be nil; check it before querying.
- **Cursor not advancing on batch failure:** Intentional by design. If one evaluation fails, the batch stops and the cursor pauses to preserve the failed event for retry.

## How to Run Locally

### Prerequisites
- Go 1.24.1+
- MongoDB/DocumentDB connection (local or remote)
- Environment variables or `.env` file (see `.env.example`)

### Configuration
Copy and customize the config:
```bash
cp .env.example .env
cp config.example.toml config.toml
# Edit .env or config.toml to set MHSGRADER_MONGO_URI, etc.
```

### Build & Run
```bash
# Build for current platform
make build

# Run locally (continues from cursor)
make run

# Run with --reset to clear state and grades, then exit
make run-reset

# Run tests
make test

# Build for Linux production
make build-linux

# Deploy to EC2 (requires SSH_HOST env var)
make deploy SSH_HOST=user@host
```

### Key Environment Variables
| Variable | Default | Description |
|----------|---------|-------------|
| `MHSGRADER_MONGO_URI` | mongodb://localhost:27017 | MongoDB/DocumentDB connection |
| `MHSGRADER_LOG_DATABASE` | stratalog | Source database (read-only) |
| `MHSGRADER_GRADES_DATABASE` | mhsgrader | Destination database (read/write) |
| `MHSGRADER_GAME` | mhs | Game identifier |
| `MHSGRADER_SCAN_INTERVAL` | 5s | Poll interval |
| `MHSGRADER_BATCH_SIZE` | 500 | Max logs per scan |
| `MHSGRADER_ACTIVE_GAP_THRESHOLD` | (check config.go) | Inactivity threshold for "active" status |

## Data Structures

### mhsgrader.progress_point_grades
Per-player aggregate document storing grades:
```js
{
  game: "mhs",
  playerId: "student@mhs.mhs",
  grades: {
    "u1p1": { color: "green", computedAt: ISODate(...), ruleId: "u1p1_v1", metrics: {} },
    "u2p3": { color: "yellow", computedAt: ISODate(...), ruleId: "u2p3_v2",
              reasonCode: "BAD_FEEDBACK", metrics: { mistakeCount: 8 } }
  },
  lastUpdated: ISODate(...)
}
```

### mhsgrader.grader_state
Single document tracking scan cursor:
```js
{
  _id: "mhs-grader",
  lastSeenId: ObjectId("..."),
  updatedAt: ISODate(...)
}
```

### stratalog.logdata (read-only source)
Events with eventKey for triggering evaluations (`user_id` is the 24-hex StrataHub user id;
`timestamp` is the client clock as a string, `serverTimestamp` is authoritative):
```js
{
  _id: ObjectId("..."),
  game: "mhs",
  user_id: "665f1a2b3c4d5e6f7a8b9c0d",
  eventType: "DialogueNodeEvent",
  eventKey: "DialogueNodeEvent:20:33",
  timestamp: "2026-09-16T01:24:30.2420000Z",
  serverTimestamp: ISODate(...),
  data: { ... }
}
```

### Testing
`go test ./internal/app/grader/ -run TestFixtureReplay -v` replays the five mhsgrading playthrough
fixtures (needs a local MongoDB and the sibling `mhsgrading` checkout or `MHSGRADING_DIR`) and checks
colours, reason codes and message variables for all 26 points; run it after every rule change.

## Related Repos

- **stratahub** — Main web app serving grades and game data; reads `mhsgrader.progress_point_grades`
- **stratalog** — Logs gameplay events into `stratalog.logdata`; mhsgrader reads these
- **waffle** — Go web framework providing logging, config, and HTTP utilities
- **mhscurriculum** — Curriculum design and rule specifications (informs rule implementation)

## Notes for Claude

1. **Read CLAUDE.md first:** This file in the root has detailed architecture notes and links to related concepts.
2. **Rule pattern is rigid:** When adding a new rule, follow the exact same structure (BaseRule, NewRule constructor, Evaluate method). Deviate only if absolutely necessary.
3. **Window queries are mandatory:** Most rules need to filter events to a (startEvent, endEvent] window. Use `LogDataHelper.CountEventsInWindow()` or similar.
4. **Test against stratalog data:** Rules depend on specific eventKey formats and data fields in logs. Verify against live or test stratalog before deploying.
5. **Versioning is evolutionary:** When updating rule logic, bump the version suffix (v1 → v2) to avoid conflicts with previously stored grades for the same unit/point.
6. **Registry is the source of truth:** The rules list in `DefaultRegistry()` controls which rules are active. Deleting or adding rules here directly affects grading.
7. **Cursor is crash-safe:** If the service crashes, it resumes from the last successful evaluation. Intentional design for idempotency.
8. **Performance:** With 5s scan intervals and 500-event batches, the service is designed for incremental, non-blocking processing. Adjust `SCAN_INTERVAL` and `BATCH_SIZE` if performance needs tuning.
