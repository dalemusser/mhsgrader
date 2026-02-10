// internal/app/rules/rule.go
// Package rules defines the grading rule interface and result types.
package rules

import (
	"context"

	"go.mongodb.org/mongo-driver/mongo"
)

// Result represents the outcome of a rule evaluation.
type Result struct {
	Color      string         // "green" or "yellow"
	ReasonCode string         // e.g., "TOO_MANY_TARGETS" (for yellow)
	Metrics    map[string]any // e.g., {countTargets: 9, threshold: 6}
}

// Green returns a green result (success).
func Green() Result {
	return Result{Color: "green"}
}

// Yellow returns a yellow result with a reason.
func Yellow(reasonCode string, metrics map[string]any) Result {
	return Result{
		Color:      "yellow",
		ReasonCode: reasonCode,
		Metrics:    metrics,
	}
}

// Rule defines the interface for a grading rule.
type Rule interface {
	// ID returns the rule's unique identifier (e.g., "u2p3_v1").
	ID() string

	// Unit returns the unit number this rule applies to.
	Unit() int

	// Point returns the progress point number within the unit.
	Point() int

	// PointID returns the combined unit/point identifier (e.g., "u2p3").
	PointID() string

	// TriggerKeys returns the eventKeys that trigger evaluation of this rule.
	TriggerKeys() []string

	// Evaluate evaluates the rule for a specific player.
	// Returns a Result indicating green/yellow and any metrics.
	Evaluate(ctx context.Context, db *mongo.Database, game, playerID string) (Result, error)
}

// BaseRule provides common functionality for rules.
type BaseRule struct {
	id          string
	unit        int
	point       int
	triggerKeys []string
}

// NewBaseRule creates a new base rule.
func NewBaseRule(unit, point int, version string, triggerKeys []string) BaseRule {
	return BaseRule{
		id:          PointIDFromUnitPoint(unit, point) + "_" + version,
		unit:        unit,
		point:       point,
		triggerKeys: triggerKeys,
	}
}

func (r BaseRule) ID() string          { return r.id }
func (r BaseRule) Unit() int           { return r.unit }
func (r BaseRule) Point() int          { return r.point }
func (r BaseRule) PointID() string     { return PointIDFromUnitPoint(r.unit, r.point) }
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
