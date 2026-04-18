// sessionpad is the host-side brain for the physical desktop workflow board.
// It connects to a Raspberry Pi Pico over USB serial, interprets button events,
// manages pending/applied config state, sends desktop notifications, and
// executes desktop actions via i3.
package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/sean/sessionpad/internal/config"
	"github.com/sean/sessionpad/internal/desktop"
	"github.com/sean/sessionpad/internal/notify"
	"github.com/sean/sessionpad/internal/protocol"
	"github.com/sean/sessionpad/internal/rules"
	"github.com/sean/sessionpad/internal/serial"
	"github.com/sean/sessionpad/internal/state"
)

func main() {
	port := flag.String("port", "/dev/ttyACM0", "serial port for the Pico")
	baud := flag.Int("baud", 115200, "baud rate")
	verbose := flag.Bool("verbose", false, "enable verbose logging")
	flag.Parse()

	log.SetFlags(log.Ltime | log.Lmicroseconds)
	log.Printf("sessionpad starting, port=%s baud=%d", *port, *baud)

	// --- Wire up components ---
	buttonMap := config.DefaultButtonMap

	mgr := state.NewManager()
	exec := desktop.NewExecutor()
	ruleEngine := rules.NewEngine()

	var notifier notify.Notifier = notify.NewNotifySend()

	// --- Connect to Pico ---
	conn, err := serial.Open(*port, *baud)
	if err != nil {
		log.Fatalf("failed to connect to Pico: %v", err)
	}
	defer conn.Close()
	log.Printf("connected to %s", *port)

	// --- Signal handling ---
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	// --- Read serial lines ---
	lines := make(chan string, 64)
	go conn.ReadLines(lines)

	// Debouncer: suppress repeated presses of the same button within a window.
	debounce := newDebouncer(200 * time.Millisecond)

	// Interaction tracker: the first toggle press in a new interaction
	// just shows current state without toggling.
	interaction := newInteractionTracker(4 * time.Second)

	log.Println("waiting for Pico events...")

	for {
		select {
		case sig := <-sigCh:
			log.Printf("received %s, shutting down", sig)
			return

		case line, ok := <-lines:
			if !ok {
				log.Println("serial connection closed")
				notifier.Notify("sessionpad", "Serial connection lost")
				return
			}

			if *verbose {
				log.Printf("rx: %s", line)
			}

			evt := protocol.Parse(line)

			// Only debounce PRESS events — let READY, RELEASE, etc. through.
			if evt.Type == protocol.EventPress {
				if !debounce.allow(evt.Button) {
					continue
				}
			}

			handleEvent(evt, buttonMap, mgr, exec, ruleEngine, notifier, interaction, *verbose)
		}
	}
}

// debouncer suppresses rapid-fire events from switch bounce.
type debouncer struct {
	window time.Duration
	mu     sync.Mutex
	last   map[string]time.Time
}

func newDebouncer(window time.Duration) *debouncer {
	return &debouncer{
		window: window,
		last:   make(map[string]time.Time),
	}
}

// allow returns true if the button press should be processed.
func (d *debouncer) allow(button string) bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	now := time.Now()
	if last, ok := d.last[button]; ok && now.Sub(last) < d.window {
		return false
	}
	d.last[button] = now
	return true
}

// interactionTracker determines whether the user is in an active
// interaction session. The first toggle press after the session
// expires just shows the current state without toggling.
type interactionTracker struct {
	timeout  time.Duration
	mu       sync.Mutex
	lastTime time.Time
}

func newInteractionTracker(timeout time.Duration) *interactionTracker {
	return &interactionTracker{timeout: timeout}
}

// touch records a toggle press and returns true if the session
// was already active (i.e. this is NOT the first press).
func (t *interactionTracker) touch() bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	now := time.Now()
	active := !t.lastTime.IsZero() && now.Sub(t.lastTime) < t.timeout
	t.lastTime = now
	return active
}

func handleEvent(
	evt protocol.Event,
	buttonMap map[string]config.ButtonAction,
	mgr *state.Manager,
	exec *desktop.Executor,
	ruleEngine *rules.Engine,
	notifier notify.Notifier,
	interaction *interactionTracker,
	verbose bool,
) {
	switch evt.Type {
	case protocol.EventReady:
		log.Println("Pico is ready")
		detected := exec.DetectRunning()
		mgr.Pending = detected.Clone()
		mgr.Applied = detected.Clone()
		log.Printf("detected running: %s", detected.Summary())
		notifyPending(mgr, buttonMap, notifier)

	case protocol.EventPong:
		if verbose {
			log.Println("pong received")
		}

	case protocol.EventState:
		if verbose {
			log.Printf("device state: %s", evt.State)
		}

	case protocol.EventRelease:
		// We act on press, not release.
		if verbose {
			log.Printf("release: %s (ignored)", evt.Button)
		}

	case protocol.EventPress:
		handlePress(evt.Button, buttonMap, mgr, exec, ruleEngine, notifier, interaction, verbose)

	case protocol.EventUnknown:
		log.Printf("unknown event: %q", evt.Raw)
	}
}

// syncWithReality re-detects running apps and updates state.
// Applied is always updated to match detected reality for detectable toggles.
// Pending is only updated for toggles the user hasn't explicitly changed.
func syncWithReality(mgr *state.Manager, exec *desktop.Executor) {
	detected := exec.DetectRunning()
	for _, name := range exec.DetectableToggles() {
		realState := detected.Toggles[name]
		if mgr.Pending.Toggles[name] == mgr.Applied.Toggles[name] {
			mgr.Pending.Toggles[name] = realState
		}
		mgr.Applied.Toggles[name] = realState
	}
}

func handlePress(
	button string,
	buttonMap map[string]config.ButtonAction,
	mgr *state.Manager,
	exec *desktop.Executor,
	ruleEngine *rules.Engine,
	notifier notify.Notifier,
	interaction *interactionTracker,
	verbose bool,
) {
	action, ok := buttonMap[button]
	if !ok {
		log.Printf("unmapped button: %s", button)
		return
	}

	switch action.Type {
	case config.Toggle:
		syncWithReality(mgr, exec)
		if !interaction.touch() {
			// First press in a new interaction — just show current state.
			log.Printf("first press (%s), showing current state", action.Name)
			notifyPending(mgr, buttonMap, notifier)
			return
		}
		nowOn := mgr.Pending.Toggle(action.Name)
		log.Printf("pending %s: %v", action.Name, nowOn)
		notifyPending(mgr, buttonMap, notifier)

	case config.Apply:
		handleApply(mgr, exec, ruleEngine, notifier, verbose)
	}
}

func handleApply(
	mgr *state.Manager,
	exec *desktop.Executor,
	ruleEngine *rules.Engine,
	notifier notify.Notifier,
	verbose bool,
) {
	log.Println("apply requested")

	// Sync with reality so the diff is accurate.
	syncWithReality(mgr, exec)

	// Check if anything changed.
	if mgr.Pending.Equal(mgr.Applied) {
		log.Println("pending == applied, nothing to do")
		notifier.Notify("sessionpad", "no changes")
		return
	}

	// Evaluate rules.
	hints := ruleEngine.Evaluate(mgr.Applied, mgr.Pending)
	for _, h := range hints {
		log.Printf("rule hint: %s", h.Description)
	}

	// Show diff.
	diff := mgr.Diff()
	log.Printf("applying changes:\n%s", diff)

	// Execute desktop actions.
	result := exec.Apply(mgr.Applied, mgr.Pending)

	if result.OK() {
		mgr.CommitPending()
		log.Printf("apply succeeded: %s", mgr.Applied.Summary())
		notifier.Notify("applied", mgr.Applied.Summary())
	} else {
		log.Printf("apply errors: %s", result.Summary())
		notifier.Notify("apply failed", result.Summary())
		// We do NOT commit on failure — pending stays dirty so user can retry.
	}
}

func notifyPending(mgr *state.Manager, buttonMap map[string]config.ButtonAction, notifier notify.Notifier) {
	body := buildStateGrid(mgr.Pending, buttonMap)
	if err := notifier.Notify("sessionpad", body); err != nil {
		log.Printf("notification error: %v", err)
	}
}

func buildStateGrid(cfg state.SessionConfig, buttonMap map[string]config.ButtonAction) string {
	var b strings.Builder
	b.WriteString("<span font_family='monospace' font_size='large'>")

	// Find the longest name across the whole grid for uniform padding.
	maxLen := 0
	for _, row := range config.GridLayout {
		for _, name := range row {
			if len(name) > maxLen {
				maxLen = len(name)
			}
		}
	}

	for _, row := range config.GridLayout {
		for j, name := range row {
			padded := fmt.Sprintf("%-*s", maxLen, name)
			if cfg.Toggles[name] {
				fmt.Fprintf(&b, "<span foreground='white'>%s</span>", padded)
			} else {
				fmt.Fprintf(&b, "<span foreground='red'>%s</span>", padded)
			}
			if j < len(row)-1 {
				b.WriteString("  ")
			}
		}
		b.WriteString("\n")
	}

	b.WriteString("</span>")
	return b.String()
}
