package platforms

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/Iotatg/iota-downloader/internal/httpx"
)

// SpotifyService resolves Spotify track links.
type SpotifyService struct {
	clientID     string
	clientSecret string
	market       string
	client       *http.Client
	log          Logger

	mu        sync.Mutex
	token     string
	tokenTill time.Time
}

// NewSpotifyService builds the Spotify resolver.
func NewSpotifyService(clientID, clientSecret, market string, log Logger) *SpotifyService {
	if market == "" {
		market = "UA"
	}
	return &SpotifyService{
		clientID:     clientID,
		clientSecret: clientSecret,
		market:       market,
		client:       httpx.NewClient(20*time.Second, 16),
		log:          log,
	}
}

// FetchTrack loads the metadata of a Spotify track.
func (s *SpotifyService) FetchTrack(ctx context.Context, rawURL string) (*TrackInfo, error) {
	kind, spotifyID, ok := ParseSpotifyURL(rawURL)
	if !ok {
		return nil, fmt.Errorf("unsupported spotify link")
	}
	if kind != "track" {
		return nil, fmt.Errorf("send a spotify track link, not an album or playlist link")
	}

	canonical := fmt.Sprintf("https://open.spotify.com/track/%s", spotifyID)
	if track, err := s.fetchFromAPI(ctx, spotifyID); err == nil {
		track.SourceURL = canonical
		return track, nil
	} else if s.log != nil {
		s.log.Debug("spotify api lookup failed: %v", err)
	}

	track, err := s.fetchFromOEmbed(ctx, canonical)
	if err != nil {
		return nil, err
	}
	track.SourceURL = canonical
	return track, nil
}

func (s *SpotifyService) accessToken(ctx context.Context) (string, error) {
	if s.clientID == "" || s.clientSecret == "" {
		return "", fmt.Errorf("spotify credentials are not configured")
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if s.token != "" && time.Now().Before(s.tokenTill) {
		return s.token, nil
	}

	credentials := base64.StdEncoding.EncodeToString([]byte(s.clientID + ":" + s.clientSecret))
	form := url.Values{"grant_type": []string{"client_credentials"}}

	request, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://accounts.spotify.com/api/token", strings.NewReader(form.Encode()))
	if err != nil {
		return "", fmt.Errorf("build spotify token request: %w", err)
	}
	request.Header.Set("Authorization", "Basic "+credentials)
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	response, err := s.client.Do(request)
	if err != nil {
		return "", fmt.Errorf("spotify token request failed: %w", err)
	}
	defer response.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(response.Body, 1<<20))
	if response.StatusCode >= 400 {
		return "", fmt.Errorf("spotify token request returned %d", response.StatusCode)
	}

	var parsed map[string]any
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return "", fmt.Errorf("decode spotify token: %w", err)
	}
	token := stringValue(parsed["access_token"])
	if token == "" {
		return "", fmt.Errorf("spotify did not return an access token")
	}
	expires := intValue(parsed["expires_in"])
	if expires < 60 {
		expires = 60
	}
	s.token = token
	s.tokenTill = time.Now().Add(time.Duration(expires-60) * time.Second)
	return token, nil
}

func (s *SpotifyService) fetchFromAPI(ctx context.Context, spotifyID string) (*TrackInfo, error) {
	token, err := s.accessToken(ctx)
	if err != nil {
		return nil, err
	}

	endpoint := "https://api.spotify.com/v1/tracks/" + url.PathEscape(spotifyID) + "?market=" + url.QueryEscape(s.market)
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("build spotify track request: %w", err)
	}
	request.Header.Set("Authorization", "Bearer "+token)

	response, err := s.client.Do(request)
	if err != nil {
		return nil, fmt.Errorf("spotify track request failed: %w", err)
	}
	defer response.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(response.Body, 4<<20))
	if response.StatusCode >= 400 {
		return nil, fmt.Errorf("spotify track request returned %d", response.StatusCode)
	}

	var parsed map[string]any
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return nil, fmt.Errorf("decode spotify track: %w", err)
	}

	track := &TrackInfo{
		ID:        "spotify:" + spotifyID,
		SpotifyID: spotifyID,
		Title:     firstNonEmpty(stringValue(parsed["name"]), "Unknown title"),
		Artists:   NormalizeArtistNames(parsed["artists"]),
		Date:      stringValue(parsed["release_date"]),
	}

	if album, ok := parsed["album"].(map[string]any); ok {
		track.Album = stringValue(album["name"])
		track.AlbumArtist = NormalizeArtistNames(album["artists"])
		if track.Artists == "" {
			track.Artists = track.AlbumArtist
		}
		if images, ok := album["images"].([]any); ok && len(images) > 0 {
			if first, ok := images[0].(map[string]any); ok {
				track.Thumbnail = HighResolutionSpotifyImage(stringValue(first["url"]))
			}
		}
	}
	if track.Artists == "" {
		track.Artists = "Unknown artist"
	}
	if track.AlbumArtist == "" {
		track.AlbumArtist = track.Artists
	}
	if duration := intValue(parsed["duration_ms"]); duration > 0 {
		track.DurationSeconds = int(float64(duration) / 1000)
	}
	if number := intValue(parsed["track_number"]); number > 0 {
		track.TrackNumber = fmt.Sprintf("%d", number)
	}
	if number := intValue(parsed["disc_number"]); number > 0 {
		track.DiscNumber = fmt.Sprintf("%d", number)
	}
	track.URL = "https://open.spotify.com/track/" + spotifyID
	return track, nil
}

var spotifyDescription = regexp.MustCompile(`(?i)\s·\s`)

func (s *SpotifyService) fetchFromOEmbed(ctx context.Context, canonical string) (*TrackInfo, error) {
	endpoint := "https://open.spotify.com/oembed?url=" + url.QueryEscape(canonical)
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("build spotify oembed request: %w", err)
	}
	request.Header.Set("User-Agent", httpx.RandomUserAgent())

	response, err := s.client.Do(request)
	if err != nil {
		return nil, fmt.Errorf("spotify oembed request failed: %w", err)
	}
	defer response.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(response.Body, 1<<20))
	if response.StatusCode >= 400 {
		return nil, fmt.Errorf("spotify oembed request returned %d", response.StatusCode)
	}

	var parsed map[string]any
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return nil, fmt.Errorf("decode spotify oembed: %w", err)
	}

	title := stringValue(parsed["title"])
	author := stringValue(parsed["author_name"])
	if title == "" {
		return nil, fmt.Errorf("spotify could not read this track")
	}

	track := &TrackInfo{
		Title:       title,
		Artists:     firstNonEmpty(author, "Unknown artist"),
		AlbumArtist: firstNonEmpty(author, "Unknown artist"),
		Thumbnail:   HighResolutionSpotifyImage(stringValue(parsed["thumbnail_url"])),
		URL:         canonical,
		SourceURL:   canonical,
	}
	return track, nil
}

// HighResolutionSpotifyImage rewrites the 300 pixel image key into the 640
// pixel one.
func HighResolutionSpotifyImage(imageURL string) string {
	if imageURL == "" {
		return ""
	}
	return strings.Replace(imageURL, "ab67616d00001e02", "ab67616d0000b273", 1)
}
