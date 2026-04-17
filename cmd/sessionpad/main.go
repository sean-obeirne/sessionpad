// sessionpad is the host-side brain for the physical desktop workflow board.
// It connects to a Raspberry Pi Pico over USB serial, interprets button events,
// manages pending/applied config state, sends desktop notifications, and
// executes desktop actions via i3.
package main

import (
	"flag"
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
	requiredGroups := []string{"editor", "context"}

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

			handleEvent(evt, buttonMap, requiredGroups, mgr, exec, ruleEngine, notifier, *verbose)
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

func handleEvent(
	evt protocol.Event,
	buttonMap map[string]config.ButtonAction,
	requiredGroups []string,
	mgr *state.Manager,
	exec *desktop.Executor,
	ruleEngine *rules.Engine,
	notifier notify.Notifier,
	verbose bool,
) {
	switch evt.Type {
	case protocol.EventReady:
		log.Println("Pico is ready")
		notifier.Notify("sessionpad", "ready")

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
		handlePress(evt.Button, buttonMap, requiredGroups, mgr, exec, ruleEngine, notifier, verbose)

	case protocol.EventUnknown:
		log.Printf("unknown event: %q", evt.Raw)
	}
}

func handlePress(
	button string,
	buttonMap map[string]config.ButtonAction,
	requiredGroups []string,
	mgr *state.Manager,
	exec *desktop.Executor,
	ruleEngine *rules.Engine,
	notifier notify.Notifier,
	verbose bool,
) {
	action, ok := buttonMap[button]
	if !ok {
		log.Printf("unmapped button: %s", button)
		return
	}

	switch action.Type {
	case config.Exclusive:
		old := mgr.Pending.Selections[action.Group]
		mgr.Pending.SetSelection(action.Group, action.Value)
		log.Printf("pending %s: %s -> %s", action.Group, old, action.Value)
		notifyPending(mgr, notifier)

	case config.Toggle:
		nowOn := mgr.Pending.ToggleExtra(action.Name)
		log.Printf("pending %s: %v", action.Name, nowOn)
		notifyPending(mgr, notifier)

	case config.Apply:
		handleApply(requiredGroups, mgr, exec, ruleEngine, notifier, verbose)
	}
}

func handleApply(
	requiredGroups []string,
	mgr *state.Manager,
	exec *desktop.Executor,
	ruleEngine *rules.Engine,
	notifier notify.Notifier,
	verbose bool,
) {
	log.Println("apply requested")

	// Validate.
	problems := mgr.Pending.Validate(requiredGroups)
	if len(problems) > 0 {
		msg := strings.Join(problems, ", ")
		log.Printf("validation failed: %v", problems)
		notifier.Notify("sessionpad", "invalid: "+msg)
		return
	}

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

func notifyPending(mgr *state.Manager, notifier notify.Notifier) {
	if err := notifier.Notify("pending", mgr.Pending.Summary()); err != nil {
		log.Printf("notification error: %v", err)
	}
}
