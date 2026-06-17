# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Commands

```sh
make gen      # regenerate internal/gen/ from api/openapi.yaml (run after every spec change)
make test     # go test -race -count=1 ./...
make lint     # golangci-lint run
make build    # compile to dist/api-proxy
make tidy     # go mod tidy

# Run a single test
go test -run TestName ./internal/media/
```

## Architecture

**Spec-first, strict server.** `api/openapi.yaml` is the single source of truth. `make gen` regenerates `internal/gen/api.gen.go` via oapi-codegen with `strict-server: true` and `std-http-server: true` (Go 1.22 net/http ServeMux, not chi). Never edit `internal/gen/`. The generated `StrictServerInterface` must be fully satisfied — the compiler enforces it.

**Request flow:**
```
net/http → RequestID → Logging → gen.Handler(strict) → jwtStrictMiddleware → handler
```

**JWT middleware is opt-out.** A `public` map in `internal/http/server.go` lists endpoints that do NOT require auth. All new endpoints are automatically JWT-protected unless explicitly added to that map.

**Type boundary.** `internal/clients/jellyfin/` returns Jellyfin-typed structs. `internal/media/mapper.go` translates them to proxy domain types (`media.Item`, `media.Detail`). HTTP handlers work only with `media.*` and `gen.*` types — never with Jellyfin types directly.

**Catalog ID format.** Proxy-scoped IDs, not Jellyfin UUIDs:
- `tmdb:movie:{id}` / `tmdb:tv:{id}` — resolved via Jellyfin's `AnyProviderIdEquals` filter
- `jf:{jellyfinUUID}` — direct Jellyfin lookup after stripping prefix

`media.Service.resolveItem()` handles translation. Never pass catalog IDs verbatim to Jellyfin.

**Playback handoff.** `media.Detail.Play` carries `JellyfinAccessToken` fetched from the `users` table via `auth.GetJellyfinToken`. Clients send all four `play.*` fields to the Jellyfin SDK.

**SQLite + embedded migrations.** `internal/db/` opens SQLite with WAL-friendly pragmas and runs all `migrations/*.sql` files at startup (embedded via `//go:embed`). SQL queries must use parameterised placeholders.

**Auth package isolation.** `internal/auth` must never import `internal/clients/jellyfin`. It owns its own `JellyfinAuthenticator` interface to stay decoupled.

## Security invariants (always check)

- JWT keyfuncs must validate the signing method before returning the key
- All SQL uses parameterised placeholders — no string concatenation
- New routes need a handler test in `internal/http/`
