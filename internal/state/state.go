// Package state manages pending and applied session configurations.
package state

import (
	"fmt"
	"sort"
	"strings"
)

// SessionConfig represents a complete board configuration.
type SessionConfig struct {
	// Toggles holds independent toggle states: name -> enabled.
	// e.g. "tmux" -> true, "runescape" -> false
	Toggles map[string]bool
}

// NewSessionConfig creates an empty config.
func NewSessionConfig() SessionConfig {
	return SessionConfig{
		Toggles: make(map[string]bool),
	}
}

// Clone returns a deep copy.
func (c SessionConfig) Clone() SessionConfig {
	out := NewSessionConfig()
	for k, v := range c.Toggles {
		out.Toggles[k] = v
	}
	return out
}

// Equal returns true if two configs are identical.
func (c SessionConfig) Equal(other SessionConfig) bool {
	if len(c.Toggles) != len(other.Toggles) {
		return false
	}
	for k, v := range c.Toggles {
		if other.Toggles[k] != v {
			return false
		}
	}
	return true
}

// Toggle flips a toggle. Returns the new state.
func (c *SessionConfig) Toggle(name string) bool {
	c.Toggles[name] = !c.Toggles[name]
	return c.Toggles[name]
}

// Enabled returns the names of all enabled toggles, sorted.
func (c SessionConfig) Enabled() []string {
	var out []string
	for k, v := range c.Toggles {
		if v {
			out = append(out, k)
		}
	}
	sort.Strings(out)
	return out
}

// Summary returns a compact human-readable summary.
func (c SessionConfig) Summary() string {
	enabled := c.Enabled()
	if len(enabled) == 0 {
		return "(empty)"
	}
	return strings.Join(enabled, " | ")
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

	allToggles := make(map[string]bool)
	for k := range m.Pending.Toggles {
		allToggles[k] = true
	}
	for k := range m.Applied.Toggles {
		allToggles[k] = true
	}
	names := make([]string, 0, len(allToggles))
	for k := range allToggles {
		names = append(names, k)
	}
	sort.Strings(names)
	for _, name := range names {
		pend := m.Pending.Toggles[name]
		app := m.Applied.Toggles[name]
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
