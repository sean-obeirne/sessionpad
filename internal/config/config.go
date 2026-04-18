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
	// Dismiss cancels the current interaction and closes the notification.
	Dismiss
)

// ButtonAction describes the semantic meaning of a single button.
type ButtonAction struct {
	Type ActionType
	Name string // for Toggle: "tmux", "runescape", etc.
}

// DefaultButtonMap is the initial mapping from physical button names to actions.
// Keys must match the button names the Pico firmware sends (e.g. "BTN_1").
// Change this map to reassign buttons.
var DefaultButtonMap = map[string]ButtonAction{
	// Row 1: toggles
	"BTN_1": {Type: Toggle, Name: "code"},
	"BTN_2": {Type: Toggle, Name: "nvim"},
	"BTN_3": {Type: Toggle, Name: "work"},
	"BTN_4": {Type: Toggle, Name: "embedded"},
	// Row 2: toggles
	"BTN_5": {Type: Toggle, Name: "firefox"},
	"BTN_6": {Type: Toggle, Name: "code"},
	"BTN_7": {Type: Toggle, Name: "runescape"},
	"BTN_8": {Type: Toggle, Name: "terminal"},
	// Row 3: toggles + apply
	"BTN_9":  {Type: Toggle, Name: "browser"},
	"BTN_10": {Type: Toggle, Name: "extra1"},
	"BTN_11": {Type: Toggle, Name: "extra2"},
	"BTN_12": {Type: Dismiss},
	"APPLY":  {Type: Apply},
}

// GridLayout defines the physical button grid as rows of toggle names.
// This determines how toggles are displayed in notifications.
var GridLayout = [][]string{
	{"embedded", "work", "nvim", "code"},
	{"terminal", "runescape", "code", "firefox"},
	{"", "extra2", "extra1", "browser"},
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
