// internal/app/rules/registry.go
package rules

import (
	"sync"
)

// Registry maps eventKeys to rules.
type Registry struct {
	mu         sync.RWMutex
	rulesByKey map[string][]Rule // eventKey -> rules (exact match)
	allRules   []Rule
	allKeys    []string // all trigger keys for scanning
}

// NewRegistry creates a new rule registry.
func NewRegistry() *Registry {
	return &Registry{
		rulesByKey: make(map[string][]Rule),
		allRules:   make([]Rule, 0),
		allKeys:    make([]string, 0),
	}
}

// Register adds a rule to the registry.
func (r *Registry) Register(rule Rule) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.allRules = append(r.allRules, rule)
	for _, key := range rule.TriggerKeys() {
		// Store with exact key for exact match lookup
		r.rulesByKey[key] = append(r.rulesByKey[key], rule)
		r.allKeys = append(r.allKeys, key)
	}
}

// GetRulesForKey returns all rules triggered by a specific eventKey.
// Uses exact string matching.
func (r *Registry) GetRulesForKey(eventKey string) []Rule {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.rulesByKey[eventKey]
}

// AllTriggerKeys returns all eventKeys that trigger rules.
func (r *Registry) AllTriggerKeys() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// Return copy of keys
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

// DefaultRegistry creates a registry with all MHS rules.
func DefaultRegistry() *Registry {
	reg := NewRegistry()

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

	return reg
}
