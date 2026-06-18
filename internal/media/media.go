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
	Genres  []string
	Runtime int // minutes
	Cast    []CastMember
	Seasons int // series only; 0 for movies
	Play    *PlayInfo
}

type PlayInfo struct {
	StreamURL string
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
