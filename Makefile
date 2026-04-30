.PHONY: build install test run clean uninstall bootstrap

BIN := bin/cpr
INSTALL_PATH := $(HOME)/.local/bin/cpr

build:
	go build -ldflags="-s -w" -o $(BIN) ./

install: build
	install -m 0755 $(BIN) $(INSTALL_PATH)
	@echo "Installed to $(INSTALL_PATH)"

test:
	go test ./... -race

run: build
	./$(BIN)

clean:
	rm -rf bin/

uninstall:
	rm -f $(INSTALL_PATH)

bootstrap:
	@command -v go >/dev/null 2>&1 || { echo "Install Go 1.22+ first: https://go.dev/dl/"; exit 1; }
	@command -v rg >/dev/null 2>&1 || echo "(optional) for fast content search: sudo apt install ripgrep"
	go mod download
	@echo "Bootstrap OK. Run 'make install' next."
