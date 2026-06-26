.PHONY: build test clean

build:
	GOOS=linux GOARCH=amd64 go build -o bin/lambda ./cmd/lambda

test:
	go test ./...

clean:
	rm -rf bin/
