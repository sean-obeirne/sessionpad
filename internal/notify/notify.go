// Package notify provides desktop notification support.
// The default implementation shells out to notify-send.
package notify

import (
	"fmt"
	"log"
	"os/exec"
	"sync"
	"time"
)

// Notifier sends desktop notifications.
type Notifier interface {
	Notify(title, body string) error
	Close() error
	Visible() bool
}

// NotifySend implements Notifier using the notify-send command.
type NotifySend struct {
	// Urgency: "low", "normal", "critical"
	Urgency string
	// ExpireMs: notification timeout in milliseconds. 0 = default.
	ExpireMs int

	mu      sync.Mutex
	shownAt time.Time
}

// NewNotifySend returns a NotifySend with sensible defaults.
func NewNotifySend() *NotifySend {
	return &NotifySend{
		Urgency:  "normal",
		ExpireMs: 4000,
	}
}

func (n *NotifySend) Notify(title, body string) error {
	args := []string{
		"--urgency", n.Urgency,
		"--expire-time", fmt.Sprintf("%d", n.ExpireMs),
		"--app-name", "sessionpad",
		"--hint", "string:x-dunst-stack-tag:sessionpad",
		title,
	}
	if body != "" {
		args = append(args, body)
	}
	cmd := exec.Command("notify-send", args...)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("notify-send: %w: %s", err, string(out))
	}
	n.mu.Lock()
	n.shownAt = time.Now()
	n.mu.Unlock()
	return nil
}

func (n *NotifySend) Close() error {
	n.mu.Lock()
	n.shownAt = time.Time{}
	n.mu.Unlock()
	cmd := exec.Command("dunstctl", "close")
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("dunstctl close: %w: %s", err, string(out))
	}
	return nil
}

func (n *NotifySend) Visible() bool {
	n.mu.Lock()
	defer n.mu.Unlock()
	if n.shownAt.IsZero() {
		return false
	}
	return time.Since(n.shownAt) < time.Duration(n.ExpireMs)*time.Millisecond
}

// LogNotifier is a fallback that just logs notifications.
type LogNotifier struct{}

func (l *LogNotifier) Notify(title, body string) error {
	log.Printf("[NOTIFY] %s: %s", title, body)
	return nil
}

func (l *LogNotifier) Close() error {
	log.Println("[NOTIFY] closed")
	return nil
}

func (l *LogNotifier) Visible() bool {
	return false
}
