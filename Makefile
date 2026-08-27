.PHONY: gen test lint build run tidy

GO ?= go
GOVERSION := $(shell awk '/^go /{print $$2}' go.mod)

# Runs inside a Linux container matching go.mod's version so the embedded-spec output (gzip,
# which encodes an OS header byte per RFC 1952) is byte-identical to what CI's Linux runner
# produces, regardless of host OS. Running natively on macOS/Windows drifts from CI's checked-in
# internal/gen/api.gen.go even when the spec itself hasn't changed.
gen:
	docker run --rm -v "$(CURDIR)":/app -w /app golang:$(GOVERSION) \
		go tool oapi-codegen -config oapi-codegen.yaml -exclude-operation-ids getStreamJfId,getStreamJfIdSubtitlesIndex,getImagesJfIdKind api/openapi.yaml

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
