.PHONY: build test lint clean install run

BIN := kubectl-srepulse
VERSION ?= dev
COMMIT  := $(shell git rev-parse --short HEAD 2>/dev/null || echo none)
DATE    := $(shell date -u +%Y-%m-%dT%H:%M:%SZ)

LDFLAGS := -s -w \
  -X 'main.version=$(VERSION)' \
  -X 'main.commit=$(COMMIT)' \
  -X 'main.date=$(DATE)'

# Build a host-native binary into ./bin/. CGO disabled so the binary
# is statically linked and works in distroless/scratch images.
build:
	CGO_ENABLED=0 go build -trimpath -ldflags "$(LDFLAGS)" \
		-o bin/$(BIN) ./cmd/kubectl-srepulse

# Convenience for the most common dev loop: build + run the TUI.
run: build
	./bin/$(BIN)

test:
	go test -race ./...

lint:
	golangci-lint run --timeout 5m ./...

clean:
	rm -rf bin/ dist/

# Install into $GOBIN so `kubectl srepulse` works (kubectl plugin
# discovery requires the binary to live on PATH).
install:
	go install -trimpath -ldflags "$(LDFLAGS)" ./cmd/kubectl-srepulse
