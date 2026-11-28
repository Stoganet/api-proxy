# stoganet

Backend API for the Stoganet client apps, in front of Jellyfin and Seerr.

## Rules

- Spec first: edit `api/openapi.yaml`, then `make gen`. Never edit `internal/gen/` by hand.
- Routes require a JWT unless listed in the `public` map in `internal/http/server.go`. Every new route gets a handler test in `internal/http/`.
- Jellyfin and Seerr types stay in `internal/clients/`. Handlers use only `media.*` and `gen.*` types.
- Catalog IDs (`tmdb:movie:{id}`, `tmdb:tv:{id}`, `jf:{uuid}`) are translated by `media.Service.resolveItem()`; never pass them to Jellyfin.
- `internal/auth` never imports `internal/clients/jellyfin`.
- The repo is public: no secrets, internal hostnames or real user data in code, tests or examples.
