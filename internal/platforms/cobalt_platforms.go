package platforms

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
)

// PinterestService resolves Pinterest pins through a Cobalt backend.
type PinterestService struct {
	cobalt *CobaltClient
	log    Logger
}

// NewPinterestService builds the Pinterest resolver.
func NewPinterestService(cobalt *CobaltClient, log Logger) *PinterestService {
	return &PinterestService{cobalt: cobalt, log: log}
}

// FetchPost resolves a Pinterest pin into a post object.
func (s *PinterestService) FetchPost(ctx context.Context, rawURL string) (*Post, error) {
	payload := map[string]any{
		"url":             rawURL,
		"downloadMode":    "auto",
		"videoQuality":    "1080",
		"alwaysProxy":     true,
		"localProcessing": "disabled",
	}

	response, err := s.cobalt.Fetch(ctx, payload, ServicePinterest)
	if err != nil {
		return nil, err
	}
	items, err := ParseItems(response, false, true)
	if err != nil {
		return nil, err
	}

	metadata := decodeMetadata(response)
	description := metadataString(metadata, "title")
	if description == "" {
		description = response.Filename
	}

	post := &Post{
		ID:          hashKey(rawURL),
		Service:     ServicePinterest,
		Description: description,
	}
	for _, item := range items {
		post.Media = append(post.Media, Media{URL: item.URL, Type: item.Type, Thumb: item.Thumb})
	}
	post.Album = len(post.Media) > 1
	return post, nil
}

// SoundCloudService resolves SoundCloud tracks through a Cobalt backend.
type SoundCloudService struct {
	cobalt *CobaltClient
	log    Logger
}

// NewSoundCloudService builds the SoundCloud resolver.
func NewSoundCloudService(cobalt *CobaltClient, log Logger) *SoundCloudService {
	return &SoundCloudService{cobalt: cobalt, log: log}
}

// FetchTrack resolves a SoundCloud track into a track object.
func (s *SoundCloudService) FetchTrack(ctx context.Context, rawURL string) (*Post, error) {
	payload := map[string]any{
		"url":             rawURL,
		"downloadMode":    "audio",
		"audioFormat":     "mp3",
		"audioBitrate":    "128",
		"alwaysProxy":     true,
		"localProcessing": "preferred",
		"disableMetadata": false,
	}

	response, err := s.cobalt.Fetch(ctx, payload, ServiceSoundCloud)
	if err != nil {
		return nil, err
	}
	items, err := ParseItems(response, true, false)
	if err != nil {
		return nil, err
	}
	if len(items) == 0 {
		return nil, fmt.Errorf("soundcloud returned no audio")
	}

	metadata := decodeMetadata(response)
	title := metadataString(metadata, "title")
	if title == "" {
		title = titleFromFilename(response.Filename)
	}
	artist := NormalizeArtistNames(metadata["artist"])
	if artist == "" {
		artist = metadataString(metadata, "uploader")
	}

	post := &Post{
		ID:      hashKey(rawURL),
		Service: ServiceSoundCloud,
		Metadata: &Metadata{
			Title:     title,
			Artists:   artist,
			Thumbnail: metadataString(metadata, "thumbnail"),
			SourceURL: rawURL,
			URL:       rawURL,
		},
	}
	if post.Metadata.Thumbnail == "" {
		post.Metadata.Thumbnail = metadataString(metadata, "cover")
	}
	post.Media = append(post.Media, Media{URL: items[0].URL, Type: KindAudio, Thumb: post.Metadata.Thumbnail})
	return post, nil
}

func decodeMetadata(response *CobaltResponse) map[string]any {
	if response == nil {
		return map[string]any{}
	}
	if len(response.Metadata) > 0 {
		var parsed map[string]any
		if err := json.Unmarshal(response.Metadata, &parsed); err == nil {
			return parsed
		}
	}
	if response.Output != nil && len(response.Output.Metadata) > 0 {
		var parsed map[string]any
		if err := json.Unmarshal(response.Output.Metadata, &parsed); err == nil {
			return parsed
		}
	}
	return map[string]any{}
}

func metadataString(metadata map[string]any, key string) string {
	if metadata == nil {
		return ""
	}
	if value, ok := metadata[key].(string); ok {
		return value
	}
	return ""
}

func authorFromFilename(filename string) string {
	if filename == "" {
		return ""
	}
	index := strings.Index(filename, "_")
	if index <= 0 {
		return ""
	}
	candidate := filename[:index]
	if len(candidate) < 2 {
		return ""
	}
	return candidate
}

func titleFromFilename(filename string) string {
	if filename == "" {
		return "SoundCloud Audio"
	}
	base := filename
	if index := strings.LastIndex(base, "."); index > 0 {
		base = base[:index]
	}
	base = strings.ReplaceAll(base, "_", " ")
	base = strings.ReplaceAll(base, "-", " ")
	base = collapseWhitespace(base)
	if base == "" {
		return "SoundCloud Audio"
	}
	return base
}

func hashKey(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])[:16]
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

// GroupOrEmpty returns the first capture group of a regular expression match
// or an empty string when the expression did not match.
func GroupOrEmpty(match []string) string {
	if len(match) > 1 {
		return match[1]
	}
	return ""
}

// ParseSpotifyURL extracts the kind and the identifier of a Spotify link.
func ParseSpotifyURL(rawURL string) (string, string, bool) {
	trimmed := strings.TrimSpace(rawURL)
	if trimmed == "" {
		return "", "", false
	}

	if strings.HasPrefix(strings.ToLower(trimmed), "spotify:") {
		parts := strings.FieldsFunc(trimmed, func(r rune) bool { return r == ':' || r == '/' })
		for index := 0; index+1 < len(parts); index++ {
			kind := strings.ToLower(parts[index])
			if kind == "track" || kind == "album" || kind == "playlist" {
				return kind, parts[index+1], true
			}
		}
		return "", "", false
	}

	parsed, err := url.Parse(trimmed)
	if err != nil {
		return "", "", false
	}
	host := strings.ToLower(parsed.Hostname())
	switch host {
	case "open.spotify.com", "spotify.com", "www.spotify.com":
	default:
		return "", "", false
	}

	segments := strings.FieldsFunc(parsed.Path, func(r rune) bool { return r == '/' })
	for index := 0; index+1 < len(segments); index++ {
		kind := strings.ToLower(segments[index])
		if kind == "track" || kind == "album" || kind == "playlist" {
			id := segments[index+1]
			if id != "" && isAlphaNumeric(id) {
				return kind, id, true
			}
		}
	}
	return "", "", false
}

func isAlphaNumeric(value string) bool {
	for _, r := range value {
		switch {
		case r >= 'a' && r <= 'z':
		case r >= 'A' && r <= 'Z':
		case r >= '0' && r <= '9':
		default:
			return false
		}
	}
	return value != ""
}
