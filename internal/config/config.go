// Package config defines the button-to-feature mapping and the semantic
// meaning of each physical button on the board.
package config

// ActionType describes what a button press does.
type ActionType int

const (
	// Toggle flips an independent boolean feature on/off.
	Toggle ActionType = iota
	// Apply triggers config application.
	Apply
)

// ButtonAction describes the semantic meaning of a single button.
type ButtonAction struct {
	Type ActionType
	Name string // for Toggle: "tmux", "runescape", etc.
}

// DefaultButtonMap is the initial mapping from physical GPIO pins to actions.
// Keys must match the button names the Pico firmware sends (e.g. "GP0").
// Change this map to reassign buttons.
var DefaultButtonMap = map[string]ButtonAction{
	// Row 1: toggles
	"GP0": {Type: Toggle, Name: "code"},
	"GP1": {Type: Toggle, Name: "nvim"},
	"GP2": {Type: Toggle, Name: "work"},
	"GP3": {Type: Toggle, Name: "embedded"},
	// Row 2: toggles
	"GP4":  {Type: Toggle, Name: "tmux"},
	"GP5":  {Type: Toggle, Name: "logs"},
	"GP10": {Type: Toggle, Name: "runescape"},
	"GP11": {Type: Toggle, Name: "music"},
	// Row 3: toggles + apply
	"GP12": {Type: Toggle, Name: "browser"},
	"GP13": {Type: Toggle, Name: "extra1"},
	"GP14": {Type: Toggle, Name: "extra2"},
	"GP15": {Type: Apply},
	"GP16": {Type: Apply},
}

// ToggleNames returns the set of toggle feature names from the button map.
func ToggleNames(bmap map[string]ButtonAction) []string {
	seen := map[string]bool{}
	var names []string
	for _, a := range bmap {
		if a.Type == Toggle && !seen[a.Name] {
			seen[a.Name] = true
			names = append(names, a.Name)
		}
	}
	return names
}
