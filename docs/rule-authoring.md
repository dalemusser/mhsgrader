# Writing a grading rule (September 2026 engine)

Each progress point is one file `internal/app/rules/uNpM.go` implementing
`Rule` through `BaseRule`. The authoritative behaviour is the point's
`mhsgrading/grading-logic/mhs-unitN-pointM-grading.md`: the **Production
Script** decides the colour, each `### CODE` under **Reason Codes** decides
whether that code fires and which message variables it returns. The Python
transcriptions `mhsgrading/rubric-validation/test_uNpM.py` (colour) and
`mhsgrading/reason-code-validation/rc_uNpM.py` (codes) are exact and validated
against five playthrough fixtures; transcribe them, do not reinterpret them.

## Skeleton

```go
type U2P1Rule struct{ BaseRule }

func NewU2P1Rule() *U2P1Rule {
	return &U2P1Rule{NewBaseRule(2, 1, "v3",
		[]string{"DialogueNodeEvent:18:1"}, // start keys: "active" state + durations
		[]string{"questFinishEvent:21"},    // end triggers: evaluation
		WithWindow(WindowPrevTrigger),      // how the script bounds the window
	)}
}

func (r *U2P1Rule) Evaluate(ctx context.Context, db *mongo.Database, game, userID string, ec EvalContext) (Result, error) {
	helper := NewLogDataHelper(db, game)
	w := ec.Window // never nil
	...
}
```

Bump the version to `v3` on every rule whose logic, keys, window or reason
codes change. Rules that are unchanged keep `v2`.

## Window kinds — pick the one the script uses

| Script pattern | Option |
|---|---|
| `latestTrigger` / `prevTrigger` on the END key (`latest_trigger_window`) | `WithWindow(WindowPrevTrigger)` |
| latest START and latest END (`start_end_window`), or latest START before the END, or START anchored by `_id < end` | `WithWindow(WindowStartBeforeEnd)` (the default) |
| the colour script also fences by client `timestamp` (U2P2, U2P3) | add `WithTimestampFence()` |

The evaluator computes `ec.Window` from that declaration. `WindowPrevTrigger`
ignores a trigger that re-fires with no new start event (no new attempt).
`ec.StartEventID` / `ec.StartTime` are the start anchor (zero/nil when none);
`ec.EndEventID` / `ec.EndTime` are the trigger. A start anchor that is not an
eventKey (the Unit-4 soil-key close) is declared with a matcher:

```go
soilKeyClose := logdata.TypeMatch("Soil Key Puzzle", map[string]any{
	"Soil Key Puzzle Status": "Finished",
	"Unit":                   UnitScene(4), // regex ^Unit 4
})
NewBaseRule(4, 2, "v3", nil, []string{"questActiveEvent:48"}, WithStartMatchers(soilKeyClose))
NewBaseRule(4, 1, "v3", []string{"DialogueNodeEvent:88:0"}, nil, WithTriggerMatchers(soilKeyClose))
```

## Helpers (all bounded to a window; nil window → empty)

- `HasEventInWindow(ctx, user, key, w)`, `HasAnyEventInWindow(ctx, user, keys, w)`
- `CountEventInIDWindow(ctx, user, key, w)`, `CountEventsInWindow(ctx, user, keys, w)`
- `FindEventsInWindow`, `EarliestEventInWindow`, `LatestEventInWindow` (by keys)
- `HasEventTypeAndData`, `CountByEventTypeAndData`, `FindByEventTypeAndData`,
  `FindEarliestByEventTypeAndData`, `FindLatestByEventTypeAndData`,
  `FindEventPairByEventTypeAndData` — `dataFilter map[string]any` with string
  values or `primitive.Regex` (`UnitScene(4)`)
- `w.Sub(startID, endID)` — an `_id`-only sub-window (phase splits, "after the latest 92:33")
- `JSRound(x)` — JavaScript `Math.round` for message numbers

## Results

- `PassedWithMetrics(metrics)`
- `FlaggedWith(metrics, Reason{Code: "...", Variables: map[string]any{...}}, ...)` —
  one `Reason` per code whose script returns `triggered: true`, in the
  markdown's order; `FlaggedWith(metrics)` with no reasons is legal for the
  yellow states the spec leaves without a code.
- `Variables` keys are the Instructor Message `{placeholders}` **verbatim**
  (`attempt_number`, `triggering_number`, `wrong_box_summary`, …) with the
  script's values: integers as `int64`, fractions as `float64`, phrases as
  `string` built word for word as the script builds them.
- `metrics` keeps the raw numbers for the analytics tab: always include
  `mistakeCount` (the point's headline error count), plus scores/counts that
  explain the colour (`score`, `posCount`, `floor3Attempts`, …).

## Verify

```bash
MHSGRADING_DIR=/Users/dale/Documents/catchupstratahub/mhsgrading \
MHSGRADER_TEST_UNITS=2 \
go test ./internal/app/grader/ -run TestFixtureReplay -v 2>&1 | grep -E "fixture .*: colors|u2p[0-9] |^(ok|FAIL)"
```

(In an agent worktree under `.claude/worktrees/`, prefix go commands with `GOWORK=off`: the workspace `go.work` does not list worktree directories.)

The replay must show every point of the unit `ok` for colour **and** for
reason codes + variables on all five fixtures. Needs a local MongoDB
(`mongodb://localhost:27017` or `MHSGRADER_TEST_MONGO_URI`).
