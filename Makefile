.PHONY: gen test lint build run tidy

GO ?= go

gen:
	$(GO) tool oapi-codegen -config oapi-codegen.yaml -exclude-operation-ids getStreamJfId api/openapi.yaml

test:
	$(GO) test -race -count=1 ./...

lint:
	golangci-lint run

build:
	CGO_ENABLED=0 $(GO) build -trimpath -ldflags="-s -w" -o dist/api-proxy ./cmd/api-proxy

run:
	$(GO) run ./cmd/api-proxy

tidy:
	$(GO) mod tidy
