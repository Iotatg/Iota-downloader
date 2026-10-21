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

// Deezer API endpoints, the api of Deezer is open and needs no key.
const (
	deezerAPIBase   = "https://api.deezer.com"
	deezerTrackLink = "https://www.deezer.com/track/"
)

// DeezerService reads the playlists and the albums of Deezer.
//
// The api of Deezer is open, it answers a playlist and its tracks without a
// key, so the bot can expand a Deezer playlist on its own.
type DeezerService struct {
	client *http.Client
	log    Logger
}

// NewDeezerService builds the client of the api of Deezer.
func NewDeezerService(log Logger) *DeezerService {
	return &DeezerService{client: httpx.NewClient(20*time.Second, 8), log: log}
}

// FetchPlaylist returns the entries of a Deezer playlist.
func (s *DeezerService) FetchPlaylist(ctx context.Context, playlistID string, limit int) (*Collection, error) {
	if strings.TrimSpace(playlistID) == "" {
		return nil, fmt.Errorf("no deezer playlist id given")
	}
	if limit <= 0 {
		limit = 50
	}

	meta, err := s.getJSON(ctx, "/playlist/"+url.PathEscape(playlistID))
	if err != nil {
		return nil, err
	}

	collection := &Collection{
		Service: ServiceDeezer,
		Kind:    CollectionPlaylist,
		ID:      playlistID,
		Title:   stringValue(meta["title"]),
		Total:   intValue(meta["nb_tracks"]),
	}

	tracks, err := s.getJSON(ctx, "/playlist/"+url.PathEscape(playlistID)+"/tracks?limit="+strconv.Itoa(limit))
	if err != nil {
		return nil, err
	}
	rows, _ := tracks["data"].([]any)
	for index, row := range rows {
		item, ok := row.(map[string]any)
		if !ok {
			continue
		}
		id := stringValue(item["id"])
		if id == "" {
			continue
		}
		artist, _ := item["artist"].(map[string]any)
		collection.Items = append(collection.Items, CollectionItem{
			Index:    index,
			Title:    stringValue(item["title"]),
			Artist:   stringValue(artist["name"]),
			URL:      deezerTrackLink + id,
			Duration: intValue(item["duration"]),
		})
	}
	if collection.Total == 0 {
		collection.Total = len(collection.Items)
	}
	return collection, nil
}

// getJSON reads a document of the api of Deezer.
func (s *DeezerService) getJSON(ctx context.Context, path string) (map[string]any, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, deezerAPIBase+path, nil)
	if err != nil {
		return nil, fmt.Errorf("build deezer request: %w", err)
	}

	response, err := s.client.Do(request)
	if err != nil {
		return nil, fmt.Errorf("deezer request failed: %w", err)
	}
	defer response.Body.Close()

	raw, _ := io.ReadAll(io.LimitReader(response.Body, 4<<20))
	if response.StatusCode >= 400 {
		return nil, fmt.Errorf("deezer returned %d", response.StatusCode)
	}

	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, fmt.Errorf("decode deezer answer: %w", err)
	}
	if message := stringValue(payload["error"]); message != "" {
		return nil, fmt.Errorf("deezer: %s", message)
	}
	return payload, nil
}
