.PHONY: build fmt lint test install

build:
	go build -o ecobeectl ./cmd/ecobeectl

install:
	go install ./cmd/ecobeectl

fmt:
	gofmt -w ./cmd ./internal

lint:
	golangci-lint run ./...

test:
	go test ./...
