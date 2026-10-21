package platforms

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/Iotatg/iota-downloader/internal/httpx"
)

// Defaults of the Ritesh api.
const (
	// DefaultRiteshAPIURL is the public address of the api.
	DefaultRiteshAPIURL = "https://api.riteshyt.in"
	// DefaultRiteshAPIKey is the free key of the public api.
	DefaultRiteshAPIKey = "riteshfreef43d064fa86c13d5f89c50f4"
)

// maxRiteshBody caps the size of an answer of the api, a hostile reply must
// not be able to exhaust the memory of the bot.
const maxRiteshBody = 8 << 20

var (
	// riteshPlayIDRe matches the full length audio stream of the api.
	riteshPlayIDRe = regexp.MustCompile(`/api/musicbot/play/([A-Za-z0-9_-]{11})`)
	// riteshVideoIDRe matches a youtube video id.
	riteshVideoIDRe = regexp.MustCompile(`^[A-Za-z0-9_-]{11}$`)
)

// RiteshService talks to the Ritesh api, a multi platform resolver that
// delivers the media of YouTube, Spotify, Apple Music, JioSaavn, Deezer,
// Gaana, Tidal and SoundCloud.
//
// The api is the fallback of the bot: it answers when the own resolver of a
// platform is blocked and it serves the music platforms that have no public
// download link.
type RiteshService struct {
	baseURL string
	apiKey  string
	client  *http.Client
	log     Logger
}

// NewRiteshService builds the client of the api.
//
// The service stays disabled when neither an address nor a key is given, an
// empty address falls back to the public api and an empty key to its free
// key.
func NewRiteshService(baseURL, apiKey string, log Logger) *RiteshService {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	apiKey = strings.TrimSpace(apiKey)
	if baseURL == "" && apiKey == "" {
		return &RiteshService{client: httpx.NewClient(45*time.Second, 8), log: log}
	}
	if baseURL == "" {
		baseURL = DefaultRiteshAPIURL
	}
	if apiKey == "" {
		apiKey = DefaultRiteshAPIKey
	}
	return &RiteshService{
		baseURL: baseURL,
		apiKey:  apiKey,
		client:  httpx.NewClient(45*time.Second, 8),
		log:     log,
	}
}

// Enabled reports whether the api can be used.
func (s *RiteshService) Enabled() bool {
	return s != nil && s.baseURL != ""
}

// RiteshFormat is one downloadable format of the api.
type RiteshFormat struct {
	Type      string
	URL       string
	DirectURL string
	Ext       string
	Quality   string
	Bitrate   float64
	HasAudio  bool
}

// RiteshExtract is the answer of the extract endpoint.
type RiteshExtract struct {
	Success     bool
	Platform    string
	ID          string
	Title       string
	Description string
	Author      string
	Thumbnail   string
	Duration    int
	MediaType   string
	Formats     []RiteshFormat
	DownloadURL string
	URL         string
}

// RiteshDetails is the answer of the details endpoint.
type RiteshDetails struct {
	ID       string
	Title    string
	Duration int
	Thumb    string
	URL      string
}

// RiteshSearchResult is one hit of the search endpoint.
type RiteshSearchResult struct {
	ID       string
	Title    string
	URL      string
	Duration int
	Thumb    string
	Channel  string
}

// Extract reads the metadata and the formats of a link.
func (s *RiteshService) Extract(ctx context.Context, rawURL string) (*RiteshExtract, error) {
	if !s.Enabled() {
		return nil, fmt.Errorf("the ritesh api is not configured")
	}
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return nil, fmt.Errorf("no link given")
	}

	raw, err := s.getJSON(ctx, "/api/platform/extract", url.Values{"url": {rawURL}})
	if err != nil {
		return nil, err
	}

	var payload struct {
		Success     bool   `json:"success"`
		Platform    string `json:"platform"`
		ID          string `json:"id"`
		Title       string `json:"title"`
		Description string `json:"description"`
		Author      struct {
			Name string `json:"name"`
		} `json:"author"`
		Thumbnail   string  `json:"thumbnail"`
		Duration    float64 `json:"duration"`
		MediaType   string  `json:"media_type"`
		DownloadURL string  `json:"download_url"`
		URL         string  `json:"url"`
		Formats     []struct {
			Type      string  `json:"type"`
			URL       string  `json:"url"`
			DirectURL string  `json:"direct_url"`
			Ext       string  `json:"ext"`
			Quality   string  `json:"quality"`
			Bitrate   float64 `json:"bitrate"`
			HasAudio  bool    `json:"has_audio"`
		} `json:"formats"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, fmt.Errorf("decode the ritesh answer: %w", err)
	}
	if !payload.Success || strings.TrimSpace(payload.Title) == "" {
		return nil, fmt.Errorf("the ritesh api returned no media for this link")
	}

	extract := &RiteshExtract{
		Success:     payload.Success,
		Platform:    payload.Platform,
		ID:          payload.ID,
		Title:       strings.TrimSpace(payload.Title),
		Description: strings.TrimSpace(payload.Description),
		Author:      strings.TrimSpace(payload.Author.Name),
		Thumbnail:   strings.TrimSpace(payload.Thumbnail),
		Duration:    int(payload.Duration + 0.5),
		MediaType:   payload.MediaType,
		DownloadURL: payload.DownloadURL,
		URL:         strings.TrimSpace(payload.URL),
	}
	for _, item := range payload.Formats {
		extract.Formats = append(extract.Formats, RiteshFormat{
			Type:      item.Type,
			URL:       item.URL,
			DirectURL: item.DirectURL,
			Ext:       item.Ext,
			Quality:   item.Quality,
			Bitrate:   item.Bitrate,
			HasAudio:  item.HasAudio,
		})
	}
	return extract, nil
}

// Details reads the metadata of a youtube video id.
func (s *RiteshService) Details(ctx context.Context, videoID string) (*RiteshDetails, error) {
	if !s.Enabled() {
		return nil, fmt.Errorf("the ritesh api is not configured")
	}
	videoID = strings.TrimSpace(videoID)
	if videoID == "" {
		return nil, fmt.Errorf("no video id given")
	}

	raw, err := s.getJSON(ctx, "/details", url.Values{"link": {videoID}})
	if err != nil {
		return nil, err
	}

	var payload struct {
		Title       string `json:"title"`
		DurationSec int    `json:"duration_sec"`
		DurationMin string `json:"duration_min"`
		Thumbnail   string `json:"thumbnail"`
		VideoID     string `json:"vidid"`
		Link        string `json:"link"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, fmt.Errorf("decode the ritesh answer: %w", err)
	}
	if payload.VideoID != videoID || strings.TrimSpace(payload.Title) == "" {
		return nil, fmt.Errorf("the ritesh api returned another video")
	}

	duration := payload.DurationSec
	if duration <= 0 {
		duration = parseRiteshDuration(payload.DurationMin)
	}
	return &RiteshDetails{
		ID:       videoID,
		Title:    strings.TrimSpace(payload.Title),
		Duration: duration,
		Thumb:    strings.TrimSpace(payload.Thumbnail),
		URL:      strings.TrimSpace(payload.Link),
	}, nil
}

// Search looks for videos that match a query.
func (s *RiteshService) Search(ctx context.Context, query string, limit int) ([]RiteshSearchResult, error) {
	if !s.Enabled() {
		return nil, fmt.Errorf("the ritesh api is not configured")
	}
	query = strings.TrimSpace(query)
	if query == "" {
		return nil, fmt.Errorf("no search query given")
	}
	if limit <= 0 {
		limit = 5
	}

	raw, err := s.getJSON(ctx, "/search", url.Values{"q": {query}, "limit": {strconv.Itoa(limit)}})
	if err != nil {
		return nil, err
	}

	var payload struct {
		Result []struct {
			ID         string `json:"id"`
			Title      string `json:"title"`
			Link       string `json:"link"`
			Duration   string `json:"duration"`
			Thumbnails []struct {
				URL string `json:"url"`
			} `json:"thumbnails"`
			Channel struct {
				Name string `json:"name"`
			} `json:"channel"`
		} `json:"result"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, fmt.Errorf("decode the ritesh answer: %w", err)
	}
	if len(payload.Result) == 0 {
		return nil, fmt.Errorf("the ritesh api found nothing")
	}

	results := make([]RiteshSearchResult, 0, len(payload.Result))
	for _, item := range payload.Result {
		if !riteshVideoIDRe.MatchString(item.ID) {
			continue
		}
		thumb := ""
		if count := len(item.Thumbnails); count > 0 {
			thumb = item.Thumbnails[count-1].URL
		}
		link := strings.TrimSpace(item.Link)
		if link == "" {
			link = "https://www.youtube.com/watch?v=" + item.ID
		}
		results = append(results, RiteshSearchResult{
			ID:       item.ID,
			Title:    strings.TrimSpace(item.Title),
			URL:      link,
			Duration: parseRiteshDuration(item.Duration),
			Thumb:    thumb,
			Channel:  strings.TrimSpace(item.Channel.Name),
		})
	}
	if len(results) == 0 {
		return nil, fmt.Errorf("the ritesh api found nothing")
	}
	return results, nil
}

// DownloadURL builds the address of a media file of the api.
//
// The id is the id of the matched video, the kind is "audio" or "video".
func (s *RiteshService) DownloadURL(id, kind string) string {
	if !s.Enabled() || strings.TrimSpace(id) == "" {
		return ""
	}
	query := url.Values{
		"query":   {strings.TrimSpace(id)},
		"dl_type": {strings.TrimSpace(kind)},
	}
	if s.apiKey != "" {
		query.Set("api_key", s.apiKey)
	}
	return s.baseURL + "/download?" + query.Encode()
}

// AudioID returns the id of the full length track of an extract.
//
// The api matches a music platform track against a video and serves that
// video as the audio of the track.
func (s *RiteshService) AudioID(extract *RiteshExtract) string {
	if extract == nil {
		return ""
	}
	for _, format := range extract.Formats {
		if !strings.EqualFold(format.Type, "audio") {
			continue
		}
		for _, candidate := range []string{format.URL, format.DirectURL} {
			if match := riteshPlayIDRe.FindStringSubmatch(candidate); match != nil {
				return match[1]
			}
		}
	}
	if extract.Platform == ServiceYouTube && riteshVideoIDRe.MatchString(extract.ID) {
		return extract.ID
	}
	return ""
}

// VideoID returns the id of the video of an extract.
func (s *RiteshService) VideoID(extract *RiteshExtract) string {
	if extract == nil {
		return ""
	}
	for _, format := range extract.Formats {
		if !strings.EqualFold(format.Type, "video") {
			continue
		}
		for _, candidate := range []string{format.URL, format.DirectURL} {
			if match := riteshPlayIDRe.FindStringSubmatch(candidate); match != nil {
				return match[1]
			}
		}
	}
	if extract.Platform == ServiceYouTube && riteshVideoIDRe.MatchString(extract.ID) {
		return extract.ID
	}
	return ""
}

// HasVideo reports whether the extract carries a video stream.
func (s *RiteshService) HasVideo(extract *RiteshExtract) bool {
	if extract == nil {
		return false
	}
	for _, format := range extract.Formats {
		if strings.EqualFold(format.Type, "video") && format.HasAudio {
			return true
		}
	}
	return false
}

// HasAudio reports whether the extract carries a full length audio stream.
func (s *RiteshService) HasAudio(extract *RiteshExtract) bool {
	return s.AudioID(extract) != ""
}

// getJSON asks the api and decodes the answer.
func (s *RiteshService) getJSON(ctx context.Context, path string, params url.Values) ([]byte, error) {
	if params == nil {
		params = url.Values{}
	}
	if s.apiKey != "" && params.Get("api_key") == "" {
		params.Set("api_key", s.apiKey)
	}

	endpoint := s.baseURL + path
	if encoded := params.Encode(); encoded != "" {
		endpoint += "?" + encoded
	}

	var lastErr error
	for attempt := 0; attempt < 2; attempt++ {
		request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
		if err != nil {
			return nil, fmt.Errorf("build the ritesh request: %w", err)
		}
		request.Header.Set("User-Agent", httpx.RandomUserAgent())
		request.Header.Set("Accept", "application/json")

		response, err := s.client.Do(request)
		if err != nil {
			return nil, fmt.Errorf("the ritesh api %s failed: %w", path, err)
		}
		body, readErr := io.ReadAll(io.LimitReader(response.Body, maxRiteshBody))
		response.Body.Close()
		if readErr != nil {
			return nil, fmt.Errorf("read the ritesh answer: %w", readErr)
		}

		switch {
		case response.StatusCode == http.StatusUnauthorized || response.StatusCode == http.StatusForbidden:
			return nil, fmt.Errorf("the ritesh api refused the request, check the api key")
		case response.StatusCode >= 500:
			// The api sits behind a proxy that answers with sporadic
			// server errors, the second try usually works.
			lastErr = fmt.Errorf("the ritesh api %s returned %d", path, response.StatusCode)
			time.Sleep(400 * time.Millisecond)
			continue
		case response.StatusCode < 200 || response.StatusCode >= 300:
			return nil, fmt.Errorf("the ritesh api %s returned %d: %s", path, response.StatusCode, riteshSnippet(body))
		}

		return body, nil
	}
	return nil, lastErr
}

// parseRiteshDuration converts "m:ss" or "h:mm:ss" into seconds.
func parseRiteshDuration(value string) int {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0
	}
	total := 0
	for _, part := range strings.Split(value, ":") {
		number, err := strconv.Atoi(strings.TrimSpace(part))
		if err != nil {
			return 0
		}
		total = total*60 + number
	}
	return total
}

// riteshSnippet shortens an answer for the error message.
func riteshSnippet(body []byte) string {
	snippet := strings.Join(strings.Fields(string(body)), " ")
	if len(snippet) > 160 {
		snippet = snippet[:160] + "..."
	}
	return snippet
}
