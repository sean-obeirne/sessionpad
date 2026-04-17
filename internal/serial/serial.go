// Package serial handles connecting to the Pico over USB serial
// and reading line-based messages from it.
package serial

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"time"

	goSerial "go.bug.st/serial"
)

// Connection manages a serial port connection to the Pico.
type Connection struct {
	portName string
	baudRate int
	port     goSerial.Port
	scanner  *bufio.Scanner
}

// Open creates a new serial connection. Call Close() when done.
func Open(portName string, baudRate int) (*Connection, error) {
	mode := &goSerial.Mode{
		BaudRate: baudRate,
		DataBits: 8,
		Parity:   goSerial.NoParity,
		StopBits: goSerial.OneStopBit,
	}

	port, err := goSerial.Open(portName, mode)
	if err != nil {
		return nil, fmt.Errorf("open serial port %s: %w", portName, err)
	}

	// Set a read timeout so we can detect disconnects.
	if err := port.SetReadTimeout(2 * time.Second); err != nil {
		port.Close()
		return nil, fmt.Errorf("set read timeout: %w", err)
	}

	scanner := bufio.NewScanner(port)

	return &Connection{
		portName: portName,
		baudRate: baudRate,
		port:     port,
		scanner:  scanner,
	}, nil
}

// ReadLine blocks until a complete line is available.
// Returns io.EOF if the port is closed or disconnected.
func (c *Connection) ReadLine() (string, error) {
	if c.scanner.Scan() {
		return c.scanner.Text(), nil
	}
	if err := c.scanner.Err(); err != nil {
		return "", fmt.Errorf("serial read: %w", err)
	}
	return "", io.EOF
}

// Write sends raw bytes to the serial port, e.g. a PING command.
func (c *Connection) Write(data []byte) error {
	_, err := c.port.Write(data)
	return err
}

// Close closes the serial port.
func (c *Connection) Close() error {
	if c.port != nil {
		return c.port.Close()
	}
	return nil
}

// ReadLines continuously reads lines from the serial port and sends them
// on the provided channel. It returns when the port is closed or an
// unrecoverable error occurs.
func (c *Connection) ReadLines(lines chan<- string) {
	for {
		line, err := c.ReadLine()
		if err != nil {
			log.Printf("serial: read error: %v", err)
			close(lines)
			return
		}
		if line != "" {
			lines <- line
		}
	}
}
