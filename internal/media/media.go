package media

type Type string

const (
	TypeMovie Type = "movie"
	TypeTV    Type = "tv"
)

type State string

const (
	StatePlayable    State = "playable"
	StateDownloading State = "downloading"
	StateRequestable State = "requestable"
)

type Item struct {
	ID           string
	Title        string
	Year         int
	Type         Type
	Poster       string
	Backdrop     string
	Thumb        string // small 16:9 card image
	ThumbHasLogo bool   // Thumb has the title printed in it
	Overview     string
	State        State
}

type Detail struct {
	Item
	Genres   []string
	Runtime  int // minutes
	Cast     []CastMember
	Seasons  []Season
	Play     *PlayInfo
	Progress *WatchProgress
	Resume   *ResumeInfo
	Start    *ResumeInfo
	Episode  *EpisodeInfo // nil unless the item is an episode of a known series
}

type PlayInfo struct {
	StreamURL      string
	SubtitleTracks []SubtitleTrack
}

type SubtitleTrack struct {
	Index      int
	Language   string
	Title      string
	Codec      string
	IsDefault  bool
	IsForced   bool
	IsExternal bool
}

type Season struct {
	Number       int
	Name         string
	Year         int
	EpisodeCount int
	Overview     string
	Poster       string
}

type WatchProgress struct {
	PositionMS int64
	Played     bool
}

type Episode struct {
	ID           string
	Number       int
	SeasonNumber int
	Title        string
	Overview     string
	Runtime      int // minutes
	Thumbnail    string
	State        State
	Play         *PlayInfo
	Progress     *WatchProgress
}

type ResumeInfo struct {
	SeasonNumber  int
	EpisodeNumber int
	EpisodeID     string
	Title         string
	Thumbnail     string
	Play          PlayInfo
	Progress      WatchProgress
}

type SeriesRef struct {
	ID    string
	Title string
}

type EpisodeInfo struct {
	Series       SeriesRef
	SeasonNumber int
	Number       int
	Next         *NextEpisode // nil when there is no next episode or the lookup failed
}

type NextEpisode struct {
	ID           string
	SeasonNumber int
	Number       int
	Title        string
	Progress     *WatchProgress
}

type CastMember struct {
	Name string
	Role string
}

type ListOpts struct {
	Type       Type // TypeMovie, TypeTV, or zero value = both
	Limit      int  // default 40, max 100
	StartIndex int
}

type ListResult struct {
	Items      []Item
	Total      int
	NextCursor string // "" if last page
}

type HomeSection struct {
	ID      string
	Items   []Item
	HasMore bool
}

type HomeResult struct {
	Sections []HomeSection
}
