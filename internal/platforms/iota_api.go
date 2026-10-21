package platforms

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/Iotatg/iota-downloader/internal/httpx"
)

// IotaAPIService talks to a self hosted iota-api instance and uses it as the
// backend for the sources that need a resolver of their own (Spotify,
// SoundCloud). The api resolves a track to a streamable media id and proxies
// the bytes, so the bot never has to touch the source platform directly.
type IotaAPIService struct {
	baseURL string
	apiKey  string
	client  *http.Client
	log     Logger
}

// NewIotaAPIService builds the iota-api client.
func NewIotaAPIService(baseURL, apiKey string, log Logger) *IotaAPIService {
	trimmed := strings.TrimRight(strings.TrimSpace(baseURL), "/")
	return &IotaAPIService{
		baseURL: trimmed,
		apiKey:  strings.TrimSpace(apiKey),
		client:  httpx.NewClient(60*time.Second, 8),
		log:     log,
	}
}

// Enabled reports whether the api is configured.
func (s *IotaAPIService) Enabled() bool {
	return s != nil && s.baseURL != ""
}

// Headers returns the auth headers of the api.
func (s *IotaAPIService) Headers() map[string]string {
	if s == nil || s.apiKey == "" {
		return nil
	}
	return map[string]string{"X-API-Key": s.apiKey}
}

// IotaTrack is the metadata of a track resolved through the api.
type IotaTrack struct {
	MediaID   string
	Title     string
	Artist    string
	Duration  int
	Thumbnail string
	MimeType  string
	Provider  string
}

// StreamURL builds the absolute stream url of a media id.
func (s *IotaAPIService) StreamURL(mediaID string) string {
	if !s.Enabled() {
		return ""
	}
	return s.baseURL + "/stream/" + url.PathEscape(mediaID)
}

// Info fetches the metadata of a media id.
func (s *IotaAPIService) Info(ctx context.Context, mediaID string) (*IotaTrack, error) {
	if !s.Enabled() {
		return nil, fmt.Errorf("the iota api is not configured")
	}
	if strings.TrimSpace(mediaID) == "" {
		return nil, fmt.Errorf("no media id given")
	}

	endpoint := s.baseURL + "/info/" + url.PathEscape(mediaID)
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("build iota api info request: %w", err)
	}
	for name, value := range s.Headers() {
		request.Header.Set(name, value)
	}

	response, err := s.client.Do(request)
	if err != nil {
		return nil, fmt.Errorf("iota api info request failed: %w", err)
	}
	defer response.Body.Close()

	raw, _ := io.ReadAll(io.LimitReader(response.Body, 1<<20))
	if response.StatusCode >= 400 {
		return nil, fmt.Errorf("iota api returned %d: %s", response.StatusCode, apiErrorMessage(raw))
	}

	var payload struct {
		MediaID   string `json:"media_id"`
		Title     string `json:"title"`
		Artist    string `json:"artist"`
		Duration  int    `json:"duration_seconds"`
		Thumbnail string `json:"thumbnail_url"`
		MimeType  string `json:"mime_type"`
		Provider  string `json:"provider"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, fmt.Errorf("decode iota api info: %w", err)
	}

	track := &IotaTrack{
		MediaID:   payload.MediaID,
		Title:     payload.Title,
		Artist:    payload.Artist,
		Duration:  payload.Duration,
		Thumbnail: payload.Thumbnail,
		MimeType:  payload.MimeType,
		Provider:  payload.Provider,
	}
	if track.MediaID == "" {
		track.MediaID = mediaID
	}
	return track, nil
}

// Search looks for tracks that match a query.
func (s *IotaAPIService) Search(ctx context.Context, query string, limit int) ([]IotaTrack, error) {
	if !s.Enabled() {
		return nil, fmt.Errorf("the iota api is not configured")
	}
	query = strings.TrimSpace(query)
	if query == "" {
		return nil, fmt.Errorf("no search query given")
	}
	if limit <= 0 {
		limit = 5
	}

	endpoint := s.baseURL + "/search?q=" + url.QueryEscape(query) + "&limit=" + strconv.Itoa(limit)
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("build iota api search request: %w", err)
	}
	for name, value := range s.Headers() {
		request.Header.Set(name, value)
	}

	response, err := s.client.Do(request)
	if err != nil {
		return nil, fmt.Errorf("iota api search request failed: %w", err)
	}
	defer response.Body.Close()

	raw, _ := io.ReadAll(io.LimitReader(response.Body, 4<<20))
	if response.StatusCode >= 400 {
		return nil, fmt.Errorf("iota api returned %d: %s", response.StatusCode, apiErrorMessage(raw))
	}

	var payload struct {
		Results []struct {
			MediaID   string `json:"media_id"`
			Title     string `json:"title"`
			Artist    string `json:"artist"`
			Duration  int    `json:"duration_seconds"`
			Thumbnail string `json:"thumbnail_url"`
			MimeType  string `json:"mime_type"`
			Provider  string `json:"provider"`
		} `json:"results"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, fmt.Errorf("decode iota api search: %w", err)
	}

	tracks := make([]IotaTrack, 0, len(payload.Results))
	for _, item := range payload.Results {
		tracks = append(tracks, IotaTrack{
			MediaID:   item.MediaID,
			Title:     item.Title,
			Artist:    item.Artist,
			Duration:  item.Duration,
			Thumbnail: item.Thumbnail,
			MimeType:  item.MimeType,
			Provider:  item.Provider,
		})
	}
	return tracks, nil
}

// IotaPlaylistItem is one entry of a playlist of the api.
type IotaPlaylistItem struct {
	MediaID  string
	Title    string
	Artist   string
	Duration int
	Thumb    string
}

// IotaPlaylist is a playlist of the api.
//
// The api knows the playlists of YouTube and the playlists of Deezer, the
// entries carry the media id of the platform they live on.
type IotaPlaylist struct {
	Source string
	ID     string
	Title  string
	Total  int
	Items  []IotaPlaylistItem
}

// Playlist fetches the entries of a playlist.
//
// The identifier is the id of the playlist, a YouTube playlist is named by
// its list id, a Deezer playlist by its number.
func (s *IotaAPIService) Playlist(ctx context.Context, playlistID string, limit int) (*IotaPlaylist, error) {
	if !s.Enabled() {
		return nil, fmt.Errorf("the iota api is not configured")
	}
	playlistID = strings.TrimSpace(playlistID)
	if playlistID == "" {
		return nil, fmt.Errorf("no playlist id given")
	}
	if limit <= 0 {
		limit = 50
	}

	endpoint := s.baseURL + "/playlists/" + url.PathEscape(playlistID) + "?limit=" + strconv.Itoa(limit)
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("build iota api playlist request: %w", err)
	}
	for name, value := range s.Headers() {
		request.Header.Set(name, value)
	}

	response, err := s.client.Do(request)
	if err != nil {
		return nil, fmt.Errorf("iota api playlist request failed: %w", err)
	}
	defer response.Body.Close()

	raw, _ := io.ReadAll(io.LimitReader(response.Body, 8<<20))
	if response.StatusCode >= 400 {
		return nil, fmt.Errorf("iota api returned %d: %s", response.StatusCode, apiErrorMessage(raw))
	}

	var payload struct {
		Source   string `json:"source"`
		Playlist string `json:"playlist_id"`
		Title    string `json:"title"`
		Count    int    `json:"count"`
		Total    int    `json:"nb_tracks"`
		Results  []struct {
			MediaID   string `json:"media_id"`
			Title     string `json:"title"`
			Artist    string `json:"artist"`
			Duration  int    `json:"duration_seconds"`
			Thumbnail string `json:"thumbnail_url"`
		} `json:"results"`
		Tracks []struct {
			ID       string `json:"id"`
			Title    string `json:"title"`
			Artist   string `json:"artist"`
			Duration int    `json:"duration_seconds"`
			Art      string `json:"art"`
		} `json:"tracks"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, fmt.Errorf("decode iota api playlist: %w", err)
	}

	playlist := &IotaPlaylist{
		Source: payload.Source,
		ID:     payload.Playlist,
		Title:  payload.Title,
		Total:  payload.Count,
	}

	for _, item := range payload.Results {
		playlist.Items = append(playlist.Items, IotaPlaylistItem{
			MediaID:  item.MediaID,
			Title:    item.Title,
			Artist:   item.Artist,
			Duration: item.Duration,
			Thumb:    item.Thumbnail,
		})
	}
	for _, item := range payload.Tracks {
		playlist.Items = append(playlist.Items, IotaPlaylistItem{
			MediaID:  item.ID,
			Title:    item.Title,
			Artist:   item.Artist,
			Duration: item.Duration,
			Thumb:    item.Art,
		})
	}
	if playlist.Total == 0 {
		playlist.Total = len(playlist.Items)
	}
	return playlist, nil
}

// apiErrorMessage extracts the error detail of an api answer.
func apiErrorMessage(raw []byte) string {
	var payload struct {
		Detail any    `json:"detail"`
		Error  string `json:"error"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return strings.TrimSpace(string(raw))
	}
	switch detail := payload.Detail.(type) {
	case string:
		if detail != "" {
			return detail
		}
	case map[string]any:
		if value, ok := detail["detail"].(string); ok && value != "" {
			return value
		}
		if value, ok := detail["error"].(string); ok && value != "" {
			return value
		}
	}
	if payload.Error != "" {
		return payload.Error
	}
	return "unknown error"
}
