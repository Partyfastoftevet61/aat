BINARY    := aat
CMD       := ./cmd/aat
SANDBOX   := aat-sandbox
SANDBOX_CMD := ./cmd/aat-sandbox
VERSION   := $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT    := $(shell git rev-parse --short HEAD 2>/dev/null || echo unknown)
DATE      := $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS   := -X github.com/gburgyan/aat/internal/version.Version=$(VERSION) \
             -X github.com/gburgyan/aat/internal/version.GitCommit=$(COMMIT) \
             -X github.com/gburgyan/aat/internal/version.BuildDate=$(DATE)

.PHONY: build cli sandbox example-shop test test-race lint fmt check clean frontend

build: frontend sandbox
	go build -ldflags "$(LDFLAGS)" -o $(BINARY) $(CMD)

# The aat binary without rebuilding the web UI. It embeds whatever
# server/web/dist holds, so from a clean checkout everything but `aat web` works.
cli:
	go build -ldflags "$(LDFLAGS)" -o $(BINARY) $(CMD)

# The demo API server; no frontend needed.
sandbox:
	go build -ldflags "$(LDFLAGS)" -o $(SANDBOX) $(SANDBOX_CMD)

# Runs examples/shop against a local sandbox, as the CI example-shop job does.
# Needs curl, jq, and free ports 8765 and 8766.
example-shop: cli sandbox
	scripts/example-shop.sh

frontend:
	cd server/web && npm install && npm run build

test: frontend
	go test ./...

test-race: frontend
	go test -race ./...

lint:
	golangci-lint run --timeout 5m

fmt:
	gofmt -s -w .

check: fmt test-race lint

clean:
	rm -f $(BINARY) $(SANDBOX)
	rm -rf server/web/dist server/web/node_modules
