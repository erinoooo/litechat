GO ?= go
BIN := litechat
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)

.PHONY: all web build dev run test test-py clean

all: build

web:
	cd web && npm ci --no-audit --no-fund && npm run build

# single static binary with the frontend embedded
build: web
	CGO_ENABLED=0 $(GO) build -trimpath -ldflags="-s -w -X main.version=$(VERSION)" -o $(BIN) .

# backend on :3000 + Vite dev server (with HMR) on :5173
dev:
	@mkdir -p web/dist && [ -f web/dist/index.html ] || echo '<!doctype html>' > web/dist/index.html
	@trap 'kill 0' EXIT; $(GO) run . & (cd web && npm run dev)

test:
	$(GO) vet ./... && $(GO) test ./...
	cd web && npx svelte-check --tsconfig ./tsconfig.json && npm run lint:ui
	$(MAKE) test-py

# Python package against a freshly built binary (frontend from the last build)
test-py:
	@mkdir -p web/dist && [ -f web/dist/index.html ] || echo '<!doctype html>' > web/dist/index.html
	CGO_ENABLED=0 $(GO) build -o $(BIN) .
	LITECHAT_BINARY=$(CURDIR)/$(BIN) PYTHONPATH=python python3 -m unittest discover -s python/tests

run: build
	./$(BIN)

clean:
	rm -rf $(BIN) web/dist
