BINARY=zm
VERSION=$(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")

.PHONY: build run install clean test lint fmt

build:
	go build -ldflags "-X main.version=$(VERSION)" -o $(BINARY) .

run:
	go run .

install:
	go install -ldflags "-X main.version=$(VERSION)" .

clean:
	rm -f $(BINARY)
	go clean

test:
	go test ./...

lint:
	golangci-lint run

fmt:
	go fmt ./...
	goimports -w .
