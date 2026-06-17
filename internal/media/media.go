package media

type Item struct {
	ID       string
	Title    string
	Year     int
	Type     string // "movie" | "tv"
	Poster   string
	Backdrop string
	Overview string
	State    string // "playable" | "downloading" | "requestable"
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
	JellyfinItemID      string
	JellyfinBaseURL     string
	JellyfinAccessToken string
	JellyfinUserID      string
}

type CastMember struct {
	Name string
	Role string
}

type ListOpts struct {
	Type       string // "movie" | "tv" | "" (both)
	Limit      int    // default 40, max 100
	StartIndex int
}

type ListResult struct {
	Items      []Item
	Total      int
	NextCursor string // "" if last page
}

type HomeSection struct {
	ID      string
	Title   string
	Items   []Item
	HasMore bool
}

type HomeResult struct {
	Sections []HomeSection
}
