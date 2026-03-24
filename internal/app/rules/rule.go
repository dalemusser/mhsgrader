// internal/app/rules/rule.go
// Package rules defines the grading rule interface and result types.
package rules

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

// Status constants for grade outcomes.
const (
	StatusActive  = "active"  // Student has started but not completed the activity
	StatusPassed  = "passed"  // Completed successfully
	StatusFlagged = "flagged" // Completed with performance concerns needing review
)

// Result represents the outcome of a rule evaluation.
type Result struct {
	Status     string         // "passed" or "flagged"
	ReasonCode string         // e.g., "TOO_MANY_TARGETS" (for flagged)
	Metrics    map[string]any // e.g., {countTargets: 9, threshold: 6}
}

// Passed returns a passed result (success).
func Passed() Result {
	return Result{Status: StatusPassed}
}

// PassedWithMetrics returns a passed result with metrics.
func PassedWithMetrics(metrics map[string]any) Result {
	return Result{Status: StatusPassed, Metrics: metrics}
}

// Flagged returns a flagged result with a reason.
func Flagged(reasonCode string, metrics map[string]any) Result {
	return Result{
		Status:     StatusFlagged,
		ReasonCode: reasonCode,
		Metrics:    metrics,
	}
}

// EvalContext provides the evaluator-computed context for a rule evaluation.
type EvalContext struct {
	Window     *AttemptWindow       // (startEvent._id, endEvent._id] — nil if no start found
	StartTime  *time.Time           // Start event serverTimestamp
	EndTime    time.Time            // End event serverTimestamp
	EndEventID primitive.ObjectID   // End event _id
}

// Rule defines the interface for a grading rule.
type Rule interface {
	// ID returns the rule's unique identifier (e.g., "u2p3_v2").
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
	// These are end-of-activity events that produce a "passed" or "flagged" grade.
	TriggerKeys() []string

	// Evaluate evaluates the rule for a specific player.
	// Returns a Result indicating passed/flagged and any metrics.
	Evaluate(ctx context.Context, db *mongo.Database, game, playerID string, ec EvalContext) (Result, error)
}

// BaseRule provides common functionality for rules.
type BaseRule struct {
	id          string
	unit        int
	point       int
	startKeys   []string
	triggerKeys []string
}

// NewBaseRule creates a new base rule.
func NewBaseRule(unit, point int, version string, startKeys, triggerKeys []string) BaseRule {
	return BaseRule{
		id:          PointIDFromUnitPoint(unit, point) + "_" + version,
		unit:        unit,
		point:       point,
		startKeys:   startKeys,
		triggerKeys: triggerKeys,
	}
}

func (r BaseRule) ID() string            { return r.id }
func (r BaseRule) Unit() int             { return r.unit }
func (r BaseRule) Point() int            { return r.point }
func (r BaseRule) PointID() string       { return PointIDFromUnitPoint(r.unit, r.point) }
func (r BaseRule) StartKeys() []string   { return r.startKeys }
func (r BaseRule) TriggerKeys() []string { return r.triggerKeys }

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
