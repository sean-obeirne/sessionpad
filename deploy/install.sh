#!/bin/bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"

echo "Installing sessionpad..."

echo "  -> building binary"
(cd "$SCRIPT_DIR/.." && go build -o sessionpad ./cmd/sessionpad/)

echo "  -> installing binary to /usr/local/bin"
sudo cp "$SCRIPT_DIR/../sessionpad" /usr/local/bin/sessionpad

echo "  -> udev rules"
sudo cp "$SCRIPT_DIR/99-sessionpad.rules" /etc/udev/rules.d/
sudo udevadm control --reload-rules

echo "  -> systemd service"
sudo cp "$SCRIPT_DIR/sessionpad.service" /etc/systemd/system/
sudo systemctl daemon-reload
sudo systemctl enable sessionpad.service

echo "Done. Unplug and replug the Pico to start."
