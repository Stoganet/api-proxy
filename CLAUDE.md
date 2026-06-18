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

**Playback handoff.** `media.Detail.Play` carries a single `StreamURL` pointing at `GET /stream/{jfId}` on this proxy. Clients hit that URL with their proxy JWT — no Jellyfin SDK or credentials needed. The stream handler fetches the Jellyfin token per request from the `users` table via `auth.GetJellyfinToken` and proxies bytes through `httputil.ReverseProxy`.

**SQLite + embedded migrations.** `internal/db/` opens SQLite with WAL-friendly pragmas and runs all `migrations/*.sql` files at startup (embedded via `//go:embed`). SQL queries must use parameterised placeholders.

**Auth package isolation.** `internal/auth` must never import `internal/clients/jellyfin`. It owns its own `JellyfinAuthenticator` interface to stay decoupled.

## Go code quality (always check)

**Hoisting.** If a function returns a handler or closure, construct expensive objects (proxies, clients, compiled regexes) in the outer function — once at startup — not inside the returned closure where they'd be rebuilt per request.

**URL building.** Use `url.JoinPath` when composing URLs from path segments. Never `fmt.Sprintf` — it does not encode special characters (`?`, `#`, `%`) in segments, which corrupts URL structure if user-controlled input is involved.

**Discarded returns.** Every `_, _` discard needs a comment or a clear structural reason. Type assertion discards (`v, _ := x.(T)`) are acceptable only when the value is guaranteed by code structure (e.g. always set by middleware before the handler runs). `RowsAffected()` discards are acceptable on SQLite when the zero case is handled explicitly on the next line.

**`Rewrite` over `Director`.** `httputil.ReverseProxy.Director` is deprecated since Go 1.20. Use `Rewrite func(*ProxyRequest)` instead.

**`url.Parse` errors.** Never ignore the error from `url.Parse` with `_`. A malformed base URL returns `(nil, err)` — any method call on nil panics.

## Security invariants (always check)

- JWT keyfuncs must validate the signing method before returning the key
- All SQL uses parameterised placeholders — no string concatenation
- New routes need a handler test in `internal/http/`
