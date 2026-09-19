.PHONY: run test test-race lint fmt vet build docker-build

run:
	go run ./cmd/api

test:
	go test ./...

test-race:
	go test -race -coverprofile=coverage.txt ./...

fmt:
	gofmt -w $$(find . -name '*.go' -not -path './vendor/*')

vet:
	go vet ./...

lint: fmt vet test

build:
	CGO_ENABLED=0 go build -trimpath -ldflags='-s -w' -o bin/agentflow-api ./cmd/api

docker-build:
	docker build -t agentflow:local .
