package media

import (
	"fmt"

	"github.com/Stoganet/api-proxy/internal/clients/jellyfin"
)

const (
	ticksPerMinute = 600_000_000
	ticksPerMS     = 10_000
)

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

func toDetail(jf jellyfin.Item, jellyfinBaseURL, proxyBaseURL string) Detail {
	cast := make([]CastMember, len(jf.People))
	for i, p := range jf.People {
		cast[i] = CastMember{Name: p.Name, Role: p.Role}
	}
	runtime := 0
	if jf.Runtime > 0 {
		runtime = int(jf.Runtime / ticksPerMinute)
	}
	return Detail{
		Item:     toItem(jf, jellyfinBaseURL),
		Genres:   jf.Genres,
		Runtime:  runtime,
		Cast:     cast,
		Seasons:  []Season{},
		Play:     &PlayInfo{StreamURL: proxyBaseURL + "/stream/" + jf.ID},
		Progress: toWatchProgress(jf.UserData),
	}
}

func toSeriesDetail(jf jellyfin.Item, jfSeasons []jellyfin.Season, nextUp *jellyfin.Episode, jellyfinBaseURL, proxyBaseURL string) Detail {
	cast := make([]CastMember, len(jf.People))
	for i, p := range jf.People {
		cast[i] = CastMember{Name: p.Name, Role: p.Role}
	}
	seasons := make([]Season, len(jfSeasons))
	for i, s := range jfSeasons {
		seasons[i] = toSeason(s, jellyfinBaseURL)
	}
	var resume *ResumeInfo
	if nextUp != nil {
		r := toResumeInfo(*nextUp, jellyfinBaseURL, proxyBaseURL)
		resume = &r
	}
	return Detail{
		Item:    toItem(jf, jellyfinBaseURL),
		Genres:  jf.Genres,
		Runtime: 0,
		Cast:    cast,
		Seasons: seasons,
		Resume:  resume,
	}
}

func toSeason(jf jellyfin.Season, jellyfinBaseURL string) Season {
	poster := ""
	if jf.PrimaryImageTag != "" {
		poster = fmt.Sprintf("%s/Items/%s/Images/Primary", jellyfinBaseURL, jf.ID)
	}
	return Season{
		Number:       jf.Number,
		Name:         jf.Name,
		Year:         jf.Year,
		EpisodeCount: jf.EpisodeCount,
		Poster:       poster,
	}
}

func toEpisode(jf jellyfin.Episode, jellyfinBaseURL, proxyBaseURL string) Episode {
	runtime := 0
	if jf.RunTimeTicks > 0 {
		runtime = int(jf.RunTimeTicks / ticksPerMinute)
	}
	thumbnail := ""
	if jf.PrimaryImageTag != "" {
		thumbnail = fmt.Sprintf("%s/Items/%s/Images/Primary", jellyfinBaseURL, jf.ID)
	}
	return Episode{
		ID:           "jf:" + jf.ID,
		Number:       jf.IndexNumber,
		SeasonNumber: jf.ParentIndexNumber,
		Title:        jf.Name,
		Overview:     jf.Overview,
		Runtime:      runtime,
		Thumbnail:    thumbnail,
		State:        StatePlayable,
		Play:         &PlayInfo{StreamURL: proxyBaseURL + "/stream/" + jf.ID},
		Progress:     toWatchProgress(jf.UserData),
	}
}

func toWatchProgress(ud jellyfin.UserData) *WatchProgress {
	if ud.PlaybackPositionTicks == 0 && !ud.Played {
		return nil
	}
	return &WatchProgress{
		PositionMS: ud.PlaybackPositionTicks / ticksPerMS,
		Played:     ud.Played,
	}
}

func toResumeInfo(jf jellyfin.Episode, jellyfinBaseURL, proxyBaseURL string) ResumeInfo {
	thumbnail := ""
	if jf.PrimaryImageTag != "" {
		thumbnail = fmt.Sprintf("%s/Items/%s/Images/Primary", jellyfinBaseURL, jf.ID)
	}
	progress := toWatchProgress(jf.UserData)
	var wp WatchProgress
	if progress != nil {
		wp = *progress
	}
	return ResumeInfo{
		SeasonNumber:  jf.ParentIndexNumber,
		EpisodeNumber: jf.IndexNumber,
		EpisodeID:     "jf:" + jf.ID,
		Title:         jf.Name,
		Thumbnail:     thumbnail,
		Play:          PlayInfo{StreamURL: proxyBaseURL + "/stream/" + jf.ID},
		Progress:      wp,
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
