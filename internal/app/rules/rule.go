// internal/app/rules/rule.go
// Package rules defines the grading rule interface and result types.
package rules

import (
	"context"
	"time"

	"github.com/dalemusser/mhsgrader/internal/app/store/logdata"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

// Status constants for grade outcomes.
const (
	StatusActive  = "active"  // Student has started but not completed the activity
	StatusPassed  = "passed"  // Completed successfully
	StatusFlagged = "flagged" // Completed with performance concerns needing review
)

// EventMatch selects log entries by exact eventKey, or by eventType plus
// data-field conditions for events that carry no eventKey (e.g. the Unit 4
// soil-key puzzle close). See logdata.Match.
type EventMatch = logdata.Match

// Reason is one triggered reason code with the variables its instructor
// message interpolates (the spec's placeholder names, e.g. "attempt_number").
type Reason struct {
	Code      string
	Variables map[string]any
}

// Result represents the outcome of a rule evaluation.
type Result struct {
	Status     string             // "passed" or "flagged"
	ReasonCode string             // first triggered code (for flagged); mirrors Reasons[0].Code
	Reasons    []Reason           // every triggered reason code, in the spec's order
	Metrics    map[string]any     // raw counts/scores behind the grade
	EAScores   map[string]EAScore // EA checkpoint scores this attempt yields (ea.go); absent = unknown
}

// WithEA attaches EA checkpoint scores to a result (nil-safe).
func (r Result) WithEA(scores map[string]EAScore) Result {
	if len(scores) > 0 {
		r.EAScores = scores
	}
	return r
}

// Passed returns a passed result (success).
func Passed() Result {
	return Result{Status: StatusPassed}
}

// PassedWithMetrics returns a passed result with metrics.
func PassedWithMetrics(metrics map[string]any) Result {
	return Result{Status: StatusPassed, Metrics: metrics}
}

// Flagged returns a flagged result with a single reason code and no message
// variables. Rules that fill instructor messages use FlaggedWith.
func Flagged(reasonCode string, metrics map[string]any) Result {
	return FlaggedWith(metrics, Reason{Code: reasonCode})
}

// FlaggedWith returns a flagged result carrying the given reasons (possibly
// none: the spec has yellow states that no reason-code script explains).
func FlaggedWith(metrics map[string]any, reasons ...Reason) Result {
	r := Result{Status: StatusFlagged, Metrics: metrics, Reasons: reasons}
	if len(reasons) > 0 {
		r.ReasonCode = reasons[0].Code
	}
	return r
}

// WindowKind selects how the evaluator bounds the graded window when a
// trigger fires. Each rule declares the kind its validated production
// script uses.
type WindowKind int

const (
	// WindowStartBeforeEnd grades (latest start anchor before the trigger,
	// trigger]; when no start anchor exists the window is unbounded below.
	WindowStartBeforeEnd WindowKind = iota

	// WindowPrevTrigger grades (previous occurrence of the trigger, trigger];
	// unbounded below for the first occurrence. A trigger that fires again
	// with no start anchor since the previous occurrence is treated as a
	// re-fire and ignored (no new attempt is recorded).
	WindowPrevTrigger
)

// WindowSpec is a rule's window declaration.
type WindowSpec struct {
	Kind WindowKind
	// TimestampFence additionally restricts windowed counts to entries whose
	// client `timestamp` lies between the start anchor's and the trigger's
	// (the U2P2/U2P3 colour scripts). Skipped when either anchor lacks one.
	TimestampFence bool
}

// EvalContext provides the evaluator-computed context for a rule evaluation.
type EvalContext struct {
	Window       *AttemptWindow     // graded window (never nil when Evaluate is called)
	StartTime    *time.Time         // start anchor serverTimestamp (nil when none found)
	StartEventID primitive.ObjectID // start anchor _id (zero when none found)
	EndTime      time.Time          // trigger serverTimestamp
	EndEventID   primitive.ObjectID // trigger _id
	WindowNote   string             // non-empty when the window had to fall back (recorded in metrics)
}

// Rule defines the interface for a grading rule.
type Rule interface {
	// ID returns the rule's unique identifier (e.g., "u2p3_v3").
	ID() string

	// Unit returns the unit number this rule applies to.
	Unit() int

	// Point returns the progress point number within the unit.
	Point() int

	// PointID returns the combined unit/point identifier (e.g., "u2p3").
	PointID() string

	// StartKeys returns the eventKeys that mark the start of this activity.
	// When scanned, these set the point's status to "active".
	StartKeys() []string

	// TriggerKeys returns the eventKeys that trigger evaluation of this rule.
	TriggerKeys() []string

	// StartMatchers returns start anchors that are not plain eventKeys.
	StartMatchers() []EventMatch

	// TriggerMatchers returns end triggers that are not plain eventKeys.
	TriggerMatchers() []EventMatch

	// Window returns how the graded window is bounded.
	Window() WindowSpec

	// Evaluate evaluates the rule for a specific player.
	Evaluate(ctx context.Context, db *mongo.Database, game, userID string, ec EvalContext) (Result, error)
}

// BaseRule provides common functionality for rules.
type BaseRule struct {
	id              string
	unit            int
	point           int
	startKeys       []string
	triggerKeys     []string
	startMatchers   []EventMatch
	triggerMatchers []EventMatch
	window          WindowSpec
}

// Option customises a BaseRule.
type Option func(*BaseRule)

// WithWindow sets the window kind.
func WithWindow(kind WindowKind) Option { return func(b *BaseRule) { b.window.Kind = kind } }

// WithTimestampFence enables the client-timestamp fence on windowed queries.
func WithTimestampFence() Option { return func(b *BaseRule) { b.window.TimestampFence = true } }

// WithStartMatchers adds start anchors that are eventType + data matches.
func WithStartMatchers(m ...EventMatch) Option {
	return func(b *BaseRule) { b.startMatchers = append(b.startMatchers, m...) }
}

// WithTriggerMatchers adds end triggers that are eventType + data matches.
func WithTriggerMatchers(m ...EventMatch) Option {
	return func(b *BaseRule) { b.triggerMatchers = append(b.triggerMatchers, m...) }
}

// NewBaseRule creates a new base rule.
func NewBaseRule(unit, point int, version string, startKeys, triggerKeys []string, opts ...Option) BaseRule {
	b := BaseRule{
		id:          PointIDFromUnitPoint(unit, point) + "_" + version,
		unit:        unit,
		point:       point,
		startKeys:   startKeys,
		triggerKeys: triggerKeys,
	}
	for _, o := range opts {
		o(&b)
	}
	return b
}

func (r BaseRule) ID() string                    { return r.id }
func (r BaseRule) Unit() int                     { return r.unit }
func (r BaseRule) Point() int                    { return r.point }
func (r BaseRule) PointID() string               { return PointIDFromUnitPoint(r.unit, r.point) }
func (r BaseRule) StartKeys() []string           { return r.startKeys }
func (r BaseRule) TriggerKeys() []string         { return r.triggerKeys }
func (r BaseRule) StartMatchers() []EventMatch   { return r.startMatchers }
func (r BaseRule) TriggerMatchers() []EventMatch { return r.triggerMatchers }
func (r BaseRule) Window() WindowSpec            { return r.window }

// PointIDFromUnitPoint creates a point ID from unit and point numbers.
func PointIDFromUnitPoint(unit, point int) string {
	return "u" + itoa(unit) + "p" + itoa(point)
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	if n < 0 {
		return "-" + itoa(-n)
	}
	s := ""
	for n > 0 {
		s = string(rune('0'+n%10)) + s
		n /= 10
	}
	return s
}
