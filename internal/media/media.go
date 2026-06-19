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
	ID       string
	Title    string
	Year     int
	Type     Type
	Poster   string
	Backdrop string
	Overview string
	State    State
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
}

type PlayInfo struct {
	StreamURL string
}

type Season struct {
	Number       int
	Name         string
	Year         int
	EpisodeCount int
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
