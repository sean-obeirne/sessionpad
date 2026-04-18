// Package rules provides a rule evaluation layer that can inspect
// the pending and applied configs plus desktop state to make
// context-aware decisions.
//
// For V1 this is minimal — it validates and returns action hints.
// The architecture is designed so you can add rules like:
//   - "if runescape is on and coding apps are open, move RS to workspace 3"
//   - "if context changes from work to embedded, close browser"
package rules

import (
	"github.com/sean/sessionpad/internal/state"
)

// Rule evaluates a condition and optionally produces ActionHints
// that modify how the desktop executor behaves.
type Rule struct {
	Name string
	Eval func(prev, next state.SessionConfig) *ActionHint
}

// ActionHint provides extra instructions for the executor.
// For now this is a placeholder — expand as needed.
type ActionHint struct {
	// Description is a human-readable note about what this hint does.
	Description string
	// TargetWorkspace overrides where an app should be placed.
	TargetWorkspace string
	// ExtraCommands are additional commands to run.
	ExtraCommands [][]string
}

// Engine holds a set of rules and evaluates them.
type Engine struct {
	Rules []Rule
}

// NewEngine returns an engine with the default V1 rules.
func NewEngine() *Engine {
	return &Engine{
		Rules: []Rule{
			{
				Name: "runescape-secondary-monitor",
				Eval: func(prev, next state.SessionConfig) *ActionHint {
					if !next.Toggles["runescape"] {
						return nil
					}
					if next.Toggles["code"] || next.Toggles["nvim"] {
						return &ActionHint{
							Description:     "runescape with editor active — consider workspace 3",
							TargetWorkspace: "3:games",
						}
					}
					return nil
				},
			},
		},
	}
}

// Evaluate runs all rules and returns any hints.
func (e *Engine) Evaluate(prev, next state.SessionConfig) []*ActionHint {
	var hints []*ActionHint
	for _, r := range e.Rules {
		if h := r.Eval(prev, next); h != nil {
			hints = append(hints, h)
		}
	}
	return hints
}
