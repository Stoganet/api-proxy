package media

import (
	"fmt"

	"github.com/Stoganet/api-proxy/internal/clients/jellyfin"
)

const ticksPerMinute = 600_000_000

func toItem(jf jellyfin.Item, baseURL string) Item {
	return Item{
		ID:       itemID(jf),
		Title:    jf.Name,
		Year:     jf.Year,
		Type:     itemType(jf.Type),
		Poster:   fmt.Sprintf("%s/Items/%s/Images/Primary", baseURL, jf.ID),
		Backdrop: backdrop(jf, baseURL),
		Overview: jf.Overview,
		State:    StatePlayable,
	}
}

func toDetail(jf jellyfin.Item, baseURL, jfToken, jfUserID string) Detail {
	cast := make([]CastMember, len(jf.People))
	for i, p := range jf.People {
		cast[i] = CastMember{Name: p.Name, Role: p.Role}
	}

	runtime := 0
	if jf.Runtime > 0 {
		runtime = int(jf.Runtime / ticksPerMinute)
	}

	seasons := 0
	if jf.Type == jellyfin.ItemTypeSeries {
		seasons = jf.ChildCount
	}

	return Detail{
		Item:    toItem(jf, baseURL),
		Genres:  jf.Genres,
		Runtime: runtime,
		Cast:    cast,
		Seasons: seasons,
		Play: &PlayInfo{
			JellyfinItemID:      jf.ID,
			JellyfinBaseURL:     baseURL,
			JellyfinAccessToken: jfToken,
			JellyfinUserID:      jfUserID,
		},
	}
}

func itemID(jf jellyfin.Item) string {
	if tmdbID, ok := jf.ProviderIDs["Tmdb"]; ok && tmdbID != "" {
		return fmt.Sprintf("tmdb:%s:%s", itemType(jf.Type), tmdbID)
	}
	return fmt.Sprintf("jf:%s", jf.ID)
}

func itemType(jfType jellyfin.ItemType) Type {
	if jfType == jellyfin.ItemTypeSeries {
		return TypeTV
	}
	return TypeMovie
}

func backdrop(jf jellyfin.Item, baseURL string) string {
	if len(jf.BackdropTags) == 0 {
		return ""
	}
	return fmt.Sprintf("%s/Items/%s/Images/Backdrop/0", baseURL, jf.ID)
}
