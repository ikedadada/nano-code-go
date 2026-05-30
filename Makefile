.PHONY: fmt test lint run run-a2a

fmt:
	gofmt -w ./cmd ./internal

test:
	go test ./...

lint:
	golangci-lint run

run:
	go run ./cmd/nano-code

run-a2a:
	go run ./cmd/nano-code-a2a
