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
)

// spotifyTrackLink is the link of a track of Spotify.
const spotifyTrackLink = "https://open.spotify.com/track/"

// FetchPlaylist returns the entries of a Spotify playlist.
//
// The api of Spotify needs the credentials of an application, the playlist
// is only readable when they are configured.
func (s *SpotifyService) FetchPlaylist(ctx context.Context, playlistID string, limit int) (*Collection, error) {
	if strings.TrimSpace(playlistID) == "" {
		return nil, fmt.Errorf("no spotify playlist id given")
	}
	if limit <= 0 {
		limit = 50
	}

	token, err := s.accessToken(ctx)
	if err != nil {
		return nil, err
	}

	endpoint := "https://api.spotify.com/v1/playlists/" + url.PathEscape(playlistID) +
		"/tracks?limit=" + strconv.Itoa(limit) + "&market=" + url.QueryEscape(s.market)
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("build spotify playlist request: %w", err)
	}
	request.Header.Set("Authorization", "Bearer "+token)

	response, err := s.client.Do(request)
	if err != nil {
		return nil, fmt.Errorf("spotify playlist request failed: %w", err)
	}
	defer response.Body.Close()

	raw, _ := io.ReadAll(io.LimitReader(response.Body, 4<<20))
	if response.StatusCode >= 400 {
		return nil, fmt.Errorf("spotify playlist request returned %d", response.StatusCode)
	}

	var payload struct {
		Name   string `json:"name"`
		Tracks struct {
			Total int `json:"total"`
			Items []struct {
				Track struct {
					ID          string `json:"id"`
					Name        string `json:"name"`
					DurationMS  int    `json:"duration_ms"`
					ExternalURL struct {
						Spotify string `json:"spotify"`
					} `json:"external_urls"`
					Artists []struct {
						Name string `json:"name"`
					} `json:"artists"`
					Album struct {
						Images []struct {
							URL string `json:"url"`
						} `json:"images"`
					} `json:"album"`
				} `json:"track"`
			} `json:"items"`
		} `json:"tracks"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, fmt.Errorf("decode spotify playlist: %w", err)
	}

	collection := &Collection{
		Service: ServiceSpotify,
		Kind:    CollectionPlaylist,
		ID:      playlistID,
		Title:   payload.Name,
		Total:   payload.Tracks.Total,
	}

	for index, item := range payload.Tracks.Items {
		track := item.Track
		if track.ID == "" {
			continue
		}
		link := track.ExternalURL.Spotify
		if link == "" {
			link = spotifyTrackLink + track.ID
		}
		artist := ""
		if len(track.Artists) > 0 {
			artist = track.Artists[0].Name
		}
		thumb := ""
		if len(track.Album.Images) > 0 {
			thumb = track.Album.Images[0].URL
		}
		collection.Items = append(collection.Items, CollectionItem{
			Index:    index,
			Title:    track.Name,
			Artist:   artist,
			URL:      link,
			Thumb:    thumb,
			Duration: track.DurationMS / 1000,
		})
	}
	if collection.Total == 0 {
		collection.Total = len(collection.Items)
	}
	return collection, nil
}
