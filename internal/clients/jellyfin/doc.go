// Package jellyfin is a thin HTTP client for the Jellyfin media server API.
//
// # Authentication
//
// Every user-context request requires two headers — both are mandatory:
//
//   - X-Emby-Authorization: MediaBrowser header identifying the client device
//     and user. Built by [authHeader].
//   - X-Emby-Token: the server API key. Without it, user-specific fields
//     (UserData, playback position) are not returned.
//
// # Error handling
//
// HTTP 404 from Jellyfin is mapped to [ErrItemNotFound]. All other non-200
// responses return a plain error with the status code. Callers in
// internal/media/service.go re-map [ErrItemNotFound] to
// media.ErrItemNotFound before it reaches HTTP handlers.
//
// # Season 0 (Specials)
//
// [Client.GetSeasons] silently drops seasons with IndexNumber == 0.
// Jellyfin places specials in season 0; clients receive only numbered seasons.
//
// # Shows vs Items
//
// Jellyfin has two separate APIs for media:
//
//   - /Items — generic item endpoint used for movies and series metadata
//     (items.go).
//   - /Shows/* — show-specific endpoints for seasons, episodes, and next-up
//     (shows.go).
//
// # Durations
//
// All durations from Jellyfin are in 100-nanosecond ticks (RunTimeTicks,
// PlaybackPositionTicks). Conversion is done in internal/media/mapper.go —
// this package does not convert units.
package jellyfin
