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
	@echo "One-time setup steps:"
	@echo "  1. Ensure Go 1.22+ is installed: go version"
	@echo "  2. make install"
	@echo "  3. Edit ~/.bashrc and remove: alias cpr='claude-projects'"
	@echo "  4. New shell, type 'cpr'."
