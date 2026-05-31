.PHONY: fmt test race lint build run run-a2a

GO ?= go
GOCACHE ?= /tmp/go-build

fmt:
	gofmt -w ./cmd ./internal

test:
	GOCACHE=$(GOCACHE) $(GO) test ./...

race:
	GOCACHE=$(GOCACHE) $(GO) test -race ./...

lint:
	files="$$(gofmt -l ./cmd ./internal)"; \
	if [ -n "$$files" ]; then \
		echo "$$files"; \
		exit 1; \
	fi
	GOCACHE=$(GOCACHE) $(GO) vet ./...

build:
	GOCACHE=$(GOCACHE) $(GO) build -o bin/nano-code ./cmd/nano-code
	GOCACHE=$(GOCACHE) $(GO) build -o bin/nano-code-a2a ./cmd/nano-code-a2a

run:
	GOCACHE=$(GOCACHE) $(GO) run ./cmd/nano-code

run-a2a:
	GOCACHE=$(GOCACHE) $(GO) run ./cmd/nano-code-a2a
