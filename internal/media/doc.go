// Package media is the domain layer between Jellyfin and HTTP handlers.
//
// # Type boundary
//
// Handlers work exclusively with [media.Item], [media.Detail], [media.Episode],
// and related types. Jellyfin-typed structs ([jellyfin.Item], [jellyfin.Season],
// etc.) must never cross into internal/http. mapper.go is the only file allowed
// to import internal/clients/jellyfin.
//
// # Catalog IDs
//
// The proxy assigns its own IDs — Jellyfin UUIDs are never exposed to clients.
//
//   - "tmdb:movie:603" / "tmdb:tv:1396" — resolved via Jellyfin's
//     AnyProviderIdEquals filter
//   - "jf:<uuid>" — direct Jellyfin lookup after stripping the prefix
//
// [Service.resolveItem] handles translation. Episode IDs are always "jf:<uuid>"
// because episodes have no stable TMDB episode ID in Jellyfin.
//
// # Movie vs series split
//
// [Service.GetItem] branches on item type:
//   - Movie → [toDetail]: sets Play and Progress; Seasons is empty slice.
//   - Series → [toSeriesDetail]: sets Seasons and Resume; no Play or Progress.
//
// # WatchProgress nil semantics
//
// [toWatchProgress] returns nil when both PlaybackPositionTicks == 0 and
// Played == false, meaning the item has never been touched. A non-nil value
// with Played == true and PositionMS == 0 means fully watched.
//
// # Tick conversions
//
// Jellyfin stores durations in 100-nanosecond ticks.
//
//   - ticksPerMS     = 10_000   (ticks → milliseconds, used for progress)
//   - ticksPerMinute = 600_000_000 (ticks → minutes, used for runtime)
package media
