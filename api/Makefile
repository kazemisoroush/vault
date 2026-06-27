.PHONY: build test lint mock clean

build:
	GOOS=linux GOARCH=amd64 go build -o bin/lambda ./cmd/lambda

test:
	go test ./...

lint:
	golangci-lint run

mock:
	go generate ./...

clean:
	rm -rf bin/
