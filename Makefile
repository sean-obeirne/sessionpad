BIN := sessionpad
PREFIX := /usr/local/bin

.PHONY: build push restart clean

build:
	go build -o $(BIN) ./cmd/sessionpad/

push: build
	sudo systemctl stop sessionpad.service || true
	sudo cp $(BIN) $(PREFIX)/$(BIN)
	sudo systemctl start sessionpad.service

restart:
	sudo systemctl restart sessionpad.service

clean:
	rm -f $(BIN)
