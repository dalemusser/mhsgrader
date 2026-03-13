// internal/app/rules/registry.go
package rules

import (
	"sync"
)

// UnitStartEvent maps a unit to its start event key.
type UnitStartEvent struct {
	UnitID   string // e.g., "unit1"
	EventKey string // e.g., "questActiveEvent:28"
}

// Registry maps eventKeys to rules and tracks unit start events.
type Registry struct {
	mu            sync.RWMutex
	startByKey    map[string][]Rule // start eventKey -> rules (sets "active")
	endByKey      map[string][]Rule // end/trigger eventKey -> rules (evaluates)
	allRules      []Rule
	allKeys       []string           // all keys (start + end) for scanning
	unitStarts    []UnitStartEvent   // unit-level start events
	unitStartKeys map[string]string  // eventKey -> unitID
}

// NewRegistry creates a new rule registry.
func NewRegistry() *Registry {
	return &Registry{
		startByKey:    make(map[string][]Rule),
		endByKey:      make(map[string][]Rule),
		allRules:      make([]Rule, 0),
		allKeys:       make([]string, 0),
		unitStarts:    make([]UnitStartEvent, 0),
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
