// Package protocol defines the message types exchanged with the Pico firmware
// and provides parsing from raw serial lines into typed events.
package protocol

import (
	"fmt"
	"strings"
)

// EventType classifies the kind of message received from the Pico.
type EventType int

const (
	EventUnknown EventType = iota
	EventReady
	EventPress
	EventRelease
	EventState
	EventPong
)

func (t EventType) String() string {
	switch t {
	case EventReady:
		return "READY"
	case EventPress:
		return "PRESS"
	case EventRelease:
		return "RELEASE"
	case EventState:
		return "STATE"
	case EventPong:
		return "PONG"
	default:
		return "UNKNOWN"
	}
}

// Event represents a single parsed message from the Pico.
type Event struct {
	Type   EventType
	Button string // e.g. "BTN_1", "APPLY" — only set for PRESS/RELEASE
	State  string // raw state bitmap — only set for STATE
	Raw    string // the original line
}

// Parse converts a raw serial line into a typed Event.
// Lines are expected in the form:
//
//	READY
//	PRESS BTN_1
//	RELEASE BTN_1
//	PRESS APPLY
//	RELEASE APPLY
//	STATE 001010011
//	PONG
func Parse(line string) Event {
	line = strings.TrimSpace(line)
	if line == "" {
		return Event{Type: EventUnknown, Raw: line}
	}

	parts := strings.SplitN(line, " ", 2)
	cmd := parts[0]
	arg := ""
	if len(parts) > 1 {
		arg = parts[1]
	}

	switch cmd {
	case "READY":
		return Event{Type: EventReady, Raw: line}
	case "PONG":
		return Event{Type: EventPong, Raw: line}
	case "PRESS":
		if arg == "" {
			return Event{Type: EventUnknown, Raw: line}
		}
		return Event{Type: EventPress, Button: arg, Raw: line}
	case "RELEASE":
		if arg == "" {
			return Event{Type: EventUnknown, Raw: line}
		}
		return Event{Type: EventRelease, Button: arg, Raw: line}
	case "STATE":
		if arg == "" {
			return Event{Type: EventUnknown, Raw: line}
		}
		return Event{Type: EventState, State: arg, Raw: line}
	default:
		return Event{Type: EventUnknown, Raw: line}
	}
}

func (e Event) String() string {
	switch e.Type {
	case EventPress, EventRelease:
		return fmt.Sprintf("%s %s", e.Type, e.Button)
	case EventState:
		return fmt.Sprintf("STATE %s", e.State)
	default:
		return e.Type.String()
	}
}
