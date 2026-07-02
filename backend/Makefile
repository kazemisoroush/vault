.PHONY: build test lint mock run

build:
	go build ./...

test:
	go test ./...

lint:
	golangci-lint run ./...

mock:
	go generate ./...

run:
	go run ./cmd/server
