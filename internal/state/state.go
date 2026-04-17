// Package state manages pending and applied session configurations.
package state

import (
	"fmt"
	"sort"
	"strings"
)

// SessionConfig represents a complete board configuration.
type SessionConfig struct {
	// Selections holds exclusive group selections: group -> value.
	// e.g. "editor" -> "nvim", "context" -> "work"
	Selections map[string]string

	// Extras holds independent toggle states: name -> enabled.
	// e.g. "tmux" -> true, "runescape" -> false
	Extras map[string]bool
}

// NewSessionConfig creates an empty config.
func NewSessionConfig() SessionConfig {
	return SessionConfig{
		Selections: make(map[string]string),
		Extras:     make(map[string]bool),
	}
}

// Clone returns a deep copy.
func (c SessionConfig) Clone() SessionConfig {
	out := NewSessionConfig()
	for k, v := range c.Selections {
		out.Selections[k] = v
	}
	for k, v := range c.Extras {
		out.Extras[k] = v
	}
	return out
}

// Equal returns true if two configs are identical.
func (c SessionConfig) Equal(other SessionConfig) bool {
	if len(c.Selections) != len(other.Selections) {
		return false
	}
	for k, v := range c.Selections {
		if other.Selections[k] != v {
			return false
		}
	}
	if len(c.Extras) != len(other.Extras) {
		return false
	}
	for k, v := range c.Extras {
		if other.Extras[k] != v {
			return false
		}
	}
	return true
}

// SetSelection sets an exclusive group to a value.
func (c *SessionConfig) SetSelection(group, value string) {
	c.Selections[group] = value
}

// ToggleExtra flips a toggle. Returns the new state.
func (c *SessionConfig) ToggleExtra(name string) bool {
	c.Extras[name] = !c.Extras[name]
	return c.Extras[name]
}

// EnabledExtras returns the names of all enabled extras, sorted.
func (c SessionConfig) EnabledExtras() []string {
	var out []string
	for k, v := range c.Extras {
		if v {
			out = append(out, k)
		}
	}
	sort.Strings(out)
	return out
}

// Summary returns a compact human-readable summary.
func (c SessionConfig) Summary() string {
	var parts []string

	groups := make([]string, 0, len(c.Selections))
	for g := range c.Selections {
		groups = append(groups, g)
	}
	sort.Strings(groups)
	for _, g := range groups {
		parts = append(parts, c.Selections[g])
	}

	extras := c.EnabledExtras()
	parts = append(parts, extras...)

	if len(parts) == 0 {
		return "(empty)"
	}
	return strings.Join(parts, " | ")
}

// Validate checks that the config has all required selections.
// Returns a list of problems (empty = valid).
func (c SessionConfig) Validate(requiredGroups []string) []string {
	var problems []string
	for _, g := range requiredGroups {
		if c.Selections[g] == "" {
			problems = append(problems, fmt.Sprintf("%s must be selected", g))
		}
	}
	return problems
}

// Manager holds both pending and applied state.
type Manager struct {
	Pending SessionConfig
	Applied SessionConfig
}

// NewManager creates a manager with empty pending and applied configs.
func NewManager() *Manager {
	return &Manager{
		Pending: NewSessionConfig(),
		Applied: NewSessionConfig(),
	}
}

// CommitPending copies the pending config into applied.
func (m *Manager) CommitPending() {
	m.Applied = m.Pending.Clone()
}

// Diff returns a human-readable description of what changed between
// applied and pending. Used for logging/notification when applying.
func (m *Manager) Diff() string {
	var changes []string

	// Check selections.
	for group, pendVal := range m.Pending.Selections {
		appVal := m.Applied.Selections[group]
		if pendVal != appVal {
			if appVal == "" {
				changes = append(changes, fmt.Sprintf("%s: (none) -> %s", group, pendVal))
			} else {
				changes = append(changes, fmt.Sprintf("%s: %s -> %s", group, appVal, pendVal))
			}
		}
	}

	// Check extras.
	allExtras := make(map[string]bool)
	for k := range m.Pending.Extras {
		allExtras[k] = true
	}
	for k := range m.Applied.Extras {
		allExtras[k] = true
	}
	names := make([]string, 0, len(allExtras))
	for k := range allExtras {
		names = append(names, k)
	}
	sort.Strings(names)
	for _, name := range names {
		pend := m.Pending.Extras[name]
		app := m.Applied.Extras[name]
		if pend != app {
			if pend {
				changes = append(changes, fmt.Sprintf("%s: off -> on", name))
			} else {
				changes = append(changes, fmt.Sprintf("%s: on -> off", name))
			}
		}
	}

	if len(changes) == 0 {
		return "no changes"
	}
	return strings.Join(changes, "\n")
}
