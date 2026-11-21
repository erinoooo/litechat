GO ?= go
BIN := litechat

.PHONY: all web build dev run test clean

all: build

web:
	cd web && npm ci --no-audit --no-fund && npm run build

# single static binary with the frontend embedded
build: web
	CGO_ENABLED=0 $(GO) build -trimpath -ldflags="-s -w" -o $(BIN) .

# backend on :3000 + Vite dev server (with HMR) on :5173
dev:
	@mkdir -p web/dist && [ -f web/dist/index.html ] || echo '<!doctype html>' > web/dist/index.html
	@trap 'kill 0' EXIT; $(GO) run . & (cd web && npm run dev)

test:
	$(GO) vet ./... && $(GO) test ./...
	cd web && npx svelte-check --tsconfig ./tsconfig.json && npm run lint:ui

run: build
	./$(BIN)

clean:
	rm -rf $(BIN) web/dist
