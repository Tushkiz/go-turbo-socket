.PHONY: build run test bench fmt vet tidy

build:
	go build ./...

run:
	go run ./cmd/server

test:
	go test ./...

bench:
	go test -bench=. ./...

fmt:
	gofmt -w $(shell go list -f '{{.Dir}}' ./...)

vet:
	go vet ./...

tidy:
	go mod tidy
