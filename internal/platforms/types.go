package platforms

import "time"

// Media kinds shared by the platform services.
const (
	KindVideo = "video"
	KindPhoto = "photo"
	KindAudio = "audio"
	// KindDocument names a media that is delivered as a file.
	KindDocument = "document"
)

// Backends name the resolver that delivered a media.
const (
	// BackendRitesh names the Ritesh api as the source of a media.
	BackendRitesh = "ritesh"
	// BackendYTDLP names yt-dlp as the source of a media, the download runs
	// with yt-dlp as well.
	BackendYTDLP = "ytdlp"
)

// Media is a single downloadable item.
type Media struct {
	URL   string
	Type  string
	Thumb string
}

// VideoInfo is the metadata of a video based post.
type VideoInfo struct {
	ID              string
	Description     string
	Author          string
	Cover           string
	Views           int64
	Likes           int64
	Comments        int64
	Shares          int64
	MusicPlayURL    string
	DurationSeconds int
	SizeHint        int64
}

// UserInfo is the metadata of a platform profile.
type UserInfo struct {
	Nickname    string
	Followers   int64
	Videos      int64
	Likes       int64
	ProfilePic  string
	Description string
}

// TrackInfo is the metadata of an audio track.
type TrackInfo struct {
	ID              string
	SpotifyID       string
	Title           string
	Artists         string
	Album           string
	AlbumArtist     string
	Date            string
	DurationSeconds int
	TrackNumber     string
	DiscNumber      string
	Thumbnail       string
	URL             string
	SourceURL       string
	AudioURL        string
}

// Metadata is the generic metadata bag handed to the media layer.
type Metadata struct {
	Title       string
	Artists     string
	Album       string
	AlbumArtist string
	Date        string
	TrackNumber string
	DiscNumber  string
	Duration    time.Duration
	Thumbnail   string
	SourceURL   string
	URL         string
}

// Post is a resolved media post of any platform.
type Post struct {
	ID          string
	Service     string
	Author      string
	Description string
	Media       []Media
	Info        *VideoInfo
	Metadata    *Metadata
	Album       bool
}
