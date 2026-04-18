// Package desktop provides a command execution layer for desktop actions.
// All shell commands live here — business logic should call Action functions
// rather than exec.Command directly.
package desktop

import (
	"fmt"
	"log"
	"os/exec"
	"strings"

	"github.com/sean/sessionpad/internal/state"
)

// Executor runs desktop actions for a given config.
// It compares the new config to the previous applied config and
// performs the necessary actions.
type Executor struct {
	// Commands maps action keys to shell commands.
	// Keys are like "editor:nvim", "toggle:tmux", etc.
	Commands map[string]Command
}

// Command represents a shell action for enabling/disabling a feature.
type Command struct {
	// Enable is run when the feature is turned on or selected.
	Enable []string
	// Disable is run when the feature is turned off or deselected.
	// May be nil if no cleanup is needed.
	Disable []string
	// Description for logging.
	Description string
}

// NewExecutor returns an executor with the default command set.
func NewExecutor() *Executor {
	return &Executor{
		Commands: map[string]Command{
			"toggle:nvim": {
				Enable:      []string{"i3-msg", "exec", "alacritty -e nvim"},
				Description: "open neovim in terminal",
			},
			"toggle:code": {
				Enable:      []string{"i3-msg", "exec", "code"},
				Description: "open VS Code",
			},
			"toggle:work": {
				Enable:      []string{"i3-msg", "workspace", "1:work"},
				Description: "switch to work workspace",
			},
			"toggle:embedded": {
				Enable:      []string{"i3-msg", "workspace", "2:embedded"},
				Description: "switch to embedded workspace",
			},
			"toggle:tmux": {
				Enable:      []string{"i3-msg", "exec", "alacritty -e tmux new-session -A -s sessionpad"},
				Description: "open tmux session",
			},
			"toggle:logs": {
				Enable:      []string{"i3-msg", "exec", "alacritty -e journalctl -f"},
				Description: "open log viewer",
			},
			"toggle:runescape": {
				Enable:      []string{"i3-msg", "exec", "flatpak run com.jagex.Launcher"},
				Description: "launch RuneScape",
			},
			"toggle:music": {
				Enable:      []string{"i3-msg", "exec", "spotify"},
				Description: "launch music player",
			},
		},
	}
}

// Result tracks what happened during an apply.
type Result struct {
	Executed []string
	Errors   []string
}

func (r Result) OK() bool {
	return len(r.Errors) == 0
}

func (r Result) Summary() string {
	var b strings.Builder
	if len(r.Executed) > 0 {
		b.WriteString("Executed:\n")
		for _, e := range r.Executed {
			fmt.Fprintf(&b, "  - %s\n", e)
		}
	}
	if len(r.Errors) > 0 {
		b.WriteString("Errors:\n")
		for _, e := range r.Errors {
			fmt.Fprintf(&b, "  - %s\n", e)
		}
	}
	return b.String()
}

// Apply executes the desktop actions needed to transition from prev to next config.
func (e *Executor) Apply(prev, next state.SessionConfig) Result {
	var result Result

	// Handle toggles.
	allToggles := make(map[string]bool)
	for k := range prev.Toggles {
		allToggles[k] = true
	}
	for k := range next.Toggles {
		allToggles[k] = true
	}

	for name := range allToggles {
		wasOn := prev.Toggles[name]
		nowOn := next.Toggles[name]
		if wasOn == nowOn {
			continue
		}

		key := "toggle:" + name
		cmd, ok := e.Commands[key]
		if !ok {
			log.Printf("desktop: no command for %s", key)
			continue
		}

		if nowOn {
			if err := run(cmd.Enable); err != nil {
				result.Errors = append(result.Errors, fmt.Sprintf("enable %s: %v", name, err))
			} else {
				result.Executed = append(result.Executed, cmd.Description)
			}
		} else if cmd.Disable != nil {
			if err := run(cmd.Disable); err != nil {
				result.Errors = append(result.Errors, fmt.Sprintf("disable %s: %v", name, err))
			} else {
				result.Executed = append(result.Executed, "disable "+cmd.Description)
			}
		}
	}

	return result
}

func run(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("empty command")
	}
	log.Printf("desktop: exec %s", strings.Join(args, " "))
	cmd := exec.Command(args[0], args[1:]...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%w: %s", err, string(out))
	}
	return nil
}
