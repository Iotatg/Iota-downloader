package platforms

import (
	"net/url"
	"regexp"
	"strings"
)

// Kinds of a collection link.
//
// A collection is a link that holds many items: a playlist of a music
// service, an album, a board of Pinterest or a radio mix of YouTube.
const (
	// CollectionPlaylist names a playlist of a platform.
	CollectionPlaylist = "playlist"
	// CollectionAlbum names an album of a platform.
	CollectionAlbum = "album"
	// CollectionBoard names a board of Pinterest.
	CollectionBoard = "board"
	// CollectionMix names a radio mix, the endless list of YouTube.
	CollectionMix = "mix"
)

// CollectionItem is one entry of a collection.
//
// The URL is the link of the entry, the normal pipeline of the bot resolves
// it, so an entry of a playlist is delivered exactly like a link that was
// sent on its own.
type CollectionItem struct {
	// Index is the position of the entry inside the collection.
	Index int
	// Title is the name of the entry.
	Title string
	// Artist is the author of the entry, it is empty for a video.
	Artist string
	// URL is the link of the entry.
	URL string
	// Thumb is the cover of the entry.
	Thumb string
	// Duration is the length of the entry in seconds.
	Duration int
}

// Collection is a link that holds many items.
type Collection struct {
	// Service is the platform of the collection.
	Service string
	// Kind is the kind of the collection.
	Kind string
	// ID is the identifier of the collection inside its platform.
	ID string
	// Title is the name of the collection.
	Title string
	// Total is the number of the entries of the collection.
	Total int
	// Items holds the entries of the collection.
	Items []CollectionItem
}

// CollectionRef is a detected collection link.
//
// It names the kind of the collection and the identifier of the collection
// inside its platform, the engines of the bot expand it into the entries.
type CollectionRef struct {
	// Service is the platform of the collection.
	Service string
	// Kind is the kind of the collection.
	Kind string
	// ID is the identifier of the collection inside its platform.
	ID string
}

// collectionPatterns are the paths of the platforms that serve a collection.
//
// The patterns are matched against the path of the link, the identifier of
// the collection is the rest of the path.
var collectionPatterns = map[string][]struct {
	kind    string
	pattern *regexp.Regexp
}{
	ServiceSpotify: {
		{CollectionPlaylist, regexp.MustCompile(`^/playlist/([A-Za-z0-9]+)`)},
		{CollectionAlbum, regexp.MustCompile(`^/album/([A-Za-z0-9]+)`)},
	},
	ServiceSoundCloud: {
		{CollectionPlaylist, regexp.MustCompile(`^/(?:[^/]+/)?sets/([^/]+)`)},
	},
	ServiceAppleMusic: {
		{CollectionPlaylist, regexp.MustCompile(`^/(?:[a-z]{2}/)?playlist/([^/]+)`)},
		{CollectionAlbum, regexp.MustCompile(`^/(?:[a-z]{2}/)?album/([^/]+)`)},
	},
	ServiceDeezer: {
		{CollectionPlaylist, regexp.MustCompile(`^/playlist/([0-9]+)`)},
		{CollectionAlbum, regexp.MustCompile(`^/album/([0-9]+)`)},
	},
	ServiceJioSaavn: {
		{CollectionPlaylist, regexp.MustCompile(`^/playlist/([^/?#]+)`)},
		{CollectionAlbum, regexp.MustCompile(`^/album/([^/?#]+)`)},
	},
	ServiceGaana: {
		{CollectionPlaylist, regexp.MustCompile(`^/playlist/([^/?#]+)`)},
	},
	ServiceTidal: {
		{CollectionPlaylist, regexp.MustCompile(`^/playlist/([0-9a-f-]+)`)},
		{CollectionAlbum, regexp.MustCompile(`^/album/([0-9a-f-]+)`)},
	},
}

// DetectCollection reports whether a link points to a collection and names
// the collection.
//
// The detection is the shared entry point of the bot, every platform that
// serves a playlist, an album or a board is listed here.
func DetectCollection(service, rawURL string) (CollectionRef, bool) {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil || parsed.Host == "" {
		return CollectionRef{}, false
	}

	switch service {
	case ServiceYouTube, "youtube_music", "youtube_audio":
		return detectYouTubeCollection(parsed)
	case ServicePinterest:
		return detectPinterestCollection(parsed)
	}

	for _, pattern := range collectionPatterns[service] {
		if match := pattern.pattern.FindStringSubmatch(parsed.Path); match != nil {
			return CollectionRef{Service: service, Kind: pattern.kind, ID: match[1]}, true
		}
	}
	return CollectionRef{}, false
}

// detectYouTubeCollection reads the list id of a YouTube link.
//
// Every YouTube link may carry a list, the mix of YouTube starts with RD
// and is a radio list that never ends.
func detectYouTubeCollection(parsed *url.URL) (CollectionRef, bool) {
	list := parsed.Query().Get("list")
	if list == "" {
		return CollectionRef{}, false
	}
	kind := CollectionPlaylist
	if strings.HasPrefix(strings.ToUpper(list), "RD") {
		kind = CollectionMix
	}
	return CollectionRef{Service: ServiceYouTube, Kind: kind, ID: list}, true
}

// pinterestBoardPattern matches a board of a Pinterest profile.
var pinterestBoardPattern = regexp.MustCompile(`^/([^/]+)/([^/]+)/?$`)

// detectPinterestCollection reads the board of a Pinterest link.
//
// A pin lives at the first level of the site, a board lives under the name
// of its owner, so a pin is never a board.
func detectPinterestCollection(parsed *url.URL) (CollectionRef, bool) {
	match := pinterestBoardPattern.FindStringSubmatch(parsed.Path)
	if match == nil {
		return CollectionRef{}, false
	}
	if strings.EqualFold(match[1], "pin") {
		return CollectionRef{}, false
	}
	return CollectionRef{
		Service: ServicePinterest,
		Kind:    CollectionBoard,
		ID:      match[1] + "/" + match[2],
	}, true
}

// CollectionReadURL returns the link a collection is read from.
//
// The radio lists of YouTube are only readable when the link carries the
// parameter that starts the radio, a mix without it is closed to the reader.
func CollectionReadURL(ref CollectionRef, rawURL string) string {
	rawURL = strings.TrimSpace(rawURL)
	if ref.Service != ServiceYouTube || ref.Kind != CollectionMix {
		return rawURL
	}

	parsed, err := url.Parse(rawURL)
	if err != nil {
		return rawURL
	}
	query := parsed.Query()
	if query.Get("playnext") != "" {
		return rawURL
	}
	query.Set("playnext", "1")
	parsed.RawQuery = query.Encode()
	return parsed.String()
}

// CollectionKindLabel returns the name of a kind of collection.
func CollectionKindLabel(kind string) string {
	switch kind {
	case CollectionPlaylist:
		return "playlist"
	case CollectionAlbum:
		return "album"
	case CollectionBoard:
		return "board"
	case CollectionMix:
		return "mix"
	default:
		return "collection"
	}
}
