# stoganet

The Stoganet core: one HTTP API for the Stoganet client apps, in front of Jellyfin and Seerr.

## Run

```sh
cp compose/.env.example compose/.env   # fill in the values
docker compose -f compose/docker-compose.yml up
```

## Configuration

| Variable | Description |
|----------|-------------|
| `JELLYFIN_URL` | Jellyfin base URL |
| `JELLYFIN_API_KEY` | Jellyfin API key |
| `SEERR_URL` | Seerr base URL |
| `SEERR_API_KEY` | Seerr API key |
| `PROXY_BASE_URL` | Public base URL of this service, used in `stream_url` |
| `JWT_SIGNING_KEY` | JWT signing key, at least 32 bytes |
| `DB_PATH` | SQLite file path |
| `LISTEN_ADDR` | Listen address, e.g. `:8080` |

## Develop

```sh
make gen    # after changing api/openapi.yaml
make test
make lint
```
