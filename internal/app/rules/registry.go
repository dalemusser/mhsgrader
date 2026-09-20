// internal/app/rules/registry.go
package rules

import (
	"sync"

	"github.com/dalemusser/mhsgrader/internal/app/store/logdata"
)

// UnitStartEvent maps a unit to its start event key.
type UnitStartEvent struct {
	UnitID   string // e.g., "unit1"
	EventKey string // e.g., "questActiveEvent:28"
}

type matcherRule struct {
	match EventMatch
	rule  Rule
}

// Registry maps eventKeys (and eventType + data matchers) to rules and
// tracks unit start events.
type Registry struct {
	mu            sync.RWMutex
	startByKey    map[string][]Rule // start eventKey -> rules (sets "active")
	endByKey      map[string][]Rule // end/trigger eventKey -> rules (evaluates)
	startMatchers []matcherRule     // start anchors that are not eventKeys
	endMatchers   []matcherRule     // end triggers that are not eventKeys
	allRules      []Rule
	allKeys       []string          // all keys (start + end + unit) for scanning
	allMatchers   []EventMatch      // all non-key anchors for scanning (deduplicated)
	unitStarts    []UnitStartEvent  // unit-level start events
	unitStartKeys map[string]string // eventKey -> unitID
}

// NewRegistry creates a new rule registry.
func NewRegistry() *Registry {
	return &Registry{
		startByKey:    make(map[string][]Rule),
		endByKey:      make(map[string][]Rule),
		unitStartKeys: make(map[string]string),
	}
}

// Register adds a rule to the registry.
func (r *Registry) Register(rule Rule) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.allRules = append(r.allRules, rule)

	for _, key := range rule.StartKeys() {
		r.startByKey[key] = append(r.startByKey[key], rule)
		r.allKeys = appendUnique(r.allKeys, key)
	}
	for _, key := range rule.TriggerKeys() {
		r.endByKey[key] = append(r.endByKey[key], rule)
		r.allKeys = appendUnique(r.allKeys, key)
	}
	for _, m := range rule.StartMatchers() {
		r.startMatchers = append(r.startMatchers, matcherRule{m, rule})
		r.allMatchers = appendUniqueMatch(r.allMatchers, m)
	}
	for _, m := range rule.TriggerMatchers() {
		r.endMatchers = append(r.endMatchers, matcherRule{m, rule})
		r.allMatchers = appendUniqueMatch(r.allMatchers, m)
	}
}

// RegisterUnitStart registers a unit-level start event.
func (r *Registry) RegisterUnitStart(unitID, eventKey string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.unitStarts = append(r.unitStarts, UnitStartEvent{UnitID: unitID, EventKey: eventKey})
	r.unitStartKeys[eventKey] = unitID
	r.allKeys = appendUnique(r.allKeys, eventKey)
}

// GetStartRulesForKey returns rules that should be set to "active" for this start key.
func (r *Registry) GetStartRulesForKey(eventKey string) []Rule {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.startByKey[eventKey]
}

// GetEndRulesForKey returns rules that should be evaluated for this end/trigger key.
func (r *Registry) GetEndRulesForKey(eventKey string) []Rule {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.endByKey[eventKey]
}

// GetStartRulesForEvent returns the rules whose start key or start matcher
// the scanned entry satisfies.
func (r *Registry) GetStartRulesForEvent(e *logdata.LogEntry) []Rule {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := append([]Rule(nil), r.startByKey[e.EventKey]...)
	for _, mr := range r.startMatchers {
		if mr.match.Matches(e) {
			out = append(out, mr.rule)
		}
	}
	return out
}

// GetEndRulesForEvent returns the rules whose trigger key or trigger matcher
// the scanned entry satisfies.
func (r *Registry) GetEndRulesForEvent(e *logdata.LogEntry) []Rule {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := append([]Rule(nil), r.endByKey[e.EventKey]...)
	for _, mr := range r.endMatchers {
		if mr.match.Matches(e) {
			out = append(out, mr.rule)
		}
	}
	return out
}

// GetUnitForStartKey returns the unit ID if this event key is a unit start event.
// Returns empty string if not a unit start key.
func (r *Registry) GetUnitForStartKey(eventKey string) string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.unitStartKeys[eventKey]
}

// AllTriggerKeys returns all eventKeys (start + end + unit) that the scanner should watch.
func (r *Registry) AllTriggerKeys() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	keys := make([]string, len(r.allKeys))
	copy(keys, r.allKeys)
	return keys
}

// AllTriggerMatchers returns the non-key anchors the scanner should watch.
func (r *Registry) AllTriggerMatchers() []EventMatch {
	r.mu.RLock()
	defer r.mu.RUnlock()
	ms := make([]EventMatch, len(r.allMatchers))
	copy(ms, r.allMatchers)
	return ms
}

// AllRules returns all registered rules.
func (r *Registry) AllRules() []Rule {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.allRules
}

// DefaultRegistry creates a registry with all MHS rules and unit start events.
func DefaultRegistry() *Registry {
	reg := NewRegistry()

	// Unit start events (from mhs-unit-start.md)
	reg.RegisterUnitStart("unit1", "questActiveEvent:28")
	reg.RegisterUnitStart("unit2", "DialogueNodeEvent:18:1")
	reg.RegisterUnitStart("unit3", "DialogueNodeEvent:10:1")
	reg.RegisterUnitStart("unit4", "DialogueNodeEvent:88:0")
	reg.RegisterUnitStart("unit5", "questActiveEvent:43")

	// Unit 1 rules
	reg.Register(NewU1P1Rule())
	reg.Register(NewU1P2Rule())
	reg.Register(NewU1P3Rule())
	reg.Register(NewU1P4Rule())

	// Unit 2 rules
	reg.Register(NewU2P1Rule())
	reg.Register(NewU2P2Rule())
	reg.Register(NewU2P3Rule())
	reg.Register(NewU2P4Rule())
	reg.Register(NewU2P5Rule())
	reg.Register(NewU2P6Rule())
	reg.Register(NewU2P7Rule())

	// Unit 3 rules
	reg.Register(NewU3P1Rule())
	reg.Register(NewU3P2Rule())
	reg.Register(NewU3P3Rule())
	reg.Register(NewU3P4Rule())
	reg.Register(NewU3P5Rule())

	// Unit 4 rules
	reg.Register(NewU4P1Rule())
	reg.Register(NewU4P2Rule())
	reg.Register(NewU4P3Rule())
	reg.Register(NewU4P4Rule())
	reg.Register(NewU4P5Rule())
	reg.Register(NewU4P6Rule())

	// Unit 5 rules
	reg.Register(NewU5P1Rule())
	reg.Register(NewU5P2Rule())
	reg.Register(NewU5P3Rule())
	reg.Register(NewU5P4Rule())

	return reg
}

func appendUnique(slice []string, item string) []string {
	for _, s := range slice {
		if s == item {
			return slice
		}
	}
	return append(slice, item)
}

func appendUniqueMatch(slice []EventMatch, m EventMatch) []EventMatch {
	for _, s := range slice {
		if s.String() == m.String() {
			return slice
		}
	}
	return append(slice, m)
}
