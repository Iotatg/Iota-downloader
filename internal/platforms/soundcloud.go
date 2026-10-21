package platforms

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/Iotatg/iota-downloader/internal/httpx"
)

// SoundCloudMetadata is the public metadata of a SoundCloud track.
type SoundCloudMetadata struct {
	Title     string
	Artist    string
	Thumbnail string
	Duration  int
}

// FetchMetadata reads the public metadata of a SoundCloud track through the
// oEmbed endpoint. It needs no account and works while the Cobalt backends
// are unreachable.
func (s *SoundCloudService) FetchMetadata(ctx context.Context, rawURL string) (*SoundCloudMetadata, error) {
	if s == nil {
		return nil, fmt.Errorf("the soundcloud service is not ready")
	}

	endpoint := "https://soundcloud.com/oembed?format=json&url=" + url.QueryEscape(rawURL)
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("build soundcloud oembed request: %w", err)
	}
	request.Header.Set("User-Agent", httpx.RandomUserAgent())

	client := httpx.NewClient(20*time.Second, 8)
	response, err := client.Do(request)
	if err != nil {
		return nil, fmt.Errorf("soundcloud oembed request failed: %w", err)
	}
	defer response.Body.Close()

	raw, _ := io.ReadAll(io.LimitReader(response.Body, 1<<20))
	if response.StatusCode >= 400 {
		return nil, fmt.Errorf("soundcloud returned %d", response.StatusCode)
	}

	var payload struct {
		Title      string `json:"title"`
		AuthorName string `json:"author_name"`
		Thumbnail  string `json:"thumbnail_url"`
		Duration   int    `json:"duration"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, fmt.Errorf("decode soundcloud oembed: %w", err)
	}
	if payload.Title == "" {
		return nil, fmt.Errorf("soundcloud did not return a title")
	}

	return &SoundCloudMetadata{
		Title:     payload.Title,
		Artist:    payload.AuthorName,
		Thumbnail: payload.Thumbnail,
		Duration:  payload.Duration,
	}, nil
}

// SearchQuery builds the search term of a track.
func (m *SoundCloudMetadata) SearchQuery() string {
	if m == nil {
		return ""
	}
	if m.Artist == "" {
		return m.Title
	}
	return m.Artist + " " + m.Title
}

// SlugMetadata builds the metadata of a track from its url. It is the
// fallback for the oEmbed endpoint when it does not answer.
//
// The last part of the path carries the title, the part before it the
// uploader of the track.
func SlugMetadata(rawURL string) *SoundCloudMetadata {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return nil
	}
	parts := strings.Split(strings.Trim(parsed.Path, "/"), "/")
	if len(parts) == 0 || parts[0] == "" {
		return nil
	}

	metadata := &SoundCloudMetadata{Artist: humaniseSlug(parts[0])}
	if len(parts) > 1 {
		metadata.Title = humaniseSlug(parts[len(parts)-1])
	}
	return metadata
}

// humaniseSlug turns a slug of a url into a readable text.
func humaniseSlug(value string) string {
	return strings.TrimSpace(strings.ReplaceAll(strings.ReplaceAll(value, "-", " "), "_", " "))
}

// MatchScore rates how close a candidate is to the wanted track. The score
// runs from 0 (nothing in common) to 1 (same title and artist).
func MatchScore(wantTitle, wantArtist, gotTitle, gotArtist string) float64 {
	titleScore := tokenOverlap(wantTitle, gotTitle)
	artistScore := tokenOverlap(wantArtist, gotArtist)
	if wantArtist == "" || gotArtist == "" {
		return titleScore
	}
	return titleScore*0.7 + artistScore*0.3
}

// tokenOverlap compares two texts by their shared words.
func tokenOverlap(want, got string) float64 {
	wantTokens := meaningfulTokens(want)
	gotTokens := meaningfulTokens(got)
	if len(wantTokens) == 0 || len(gotTokens) == 0 {
		return 0
	}

	shared := 0
	for token := range wantTokens {
		if _, ok := gotTokens[token]; ok {
			shared++
		}
	}
	return float64(shared) / float64(len(wantTokens))
}

// meaningfulTokens splits a text into the words that identify a track, the
// noise words of a title are dropped.
func meaningfulTokens(value string) map[string]struct{} {
	tokens := map[string]struct{}{}
	for _, token := range strings.Fields(strings.ToLower(value)) {
		token = strings.Trim(token, ".,!?'\"()[]{}:-_")
		if token == "" || noiseWords[token] {
			continue
		}
		tokens[token] = struct{}{}
	}
	return tokens
}

// noiseWords are the words that carry no information about a track.
var noiseWords = map[string]bool{
	"the": true, "a": true, "an": true, "and": true, "or": true,
	"feat": true, "featuring": true, "ft": true, "with": true,
	"remix": true, "remaster": true, "remastered": true, "version": true,
	"edit": true, "mix": true, "official": true, "video": true,
	"audio": true, "lyrics": true, "lyric": true, "hd": true, "hq": true,
	"explicit": true, "clean": true, "mono": true, "stereo": true,
}
