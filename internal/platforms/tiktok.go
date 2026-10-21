package platforms

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/Iotatg/iota-downloader/internal/httpx"
)

const tikwmAPIURL = "https://tikwm.com/api/"

// TikTokService resolves TikTok posts and profiles.
type TikTokService struct {
	client *http.Client
	log    Logger
}

// NewTikTokService builds the TikTok resolver.
func NewTikTokService(log Logger) *TikTokService {
	return &TikTokService{client: httpx.NewClient(15*time.Second, 32), log: log}
}

var tiktokTrackingParams = regexp.MustCompile(`(?i)^(utm_|tt_|_?t|share_|is_from_webapp|user_id|sender_device|web_id|sdk_version|priority|region|locale|device_id|device_platform|app_name|app_version|enter_from|channel|os_version|refer|verify|vid|sec_uid|share_token|share_link_id|source|preview_player)($|_)`)

// StripTikTokTracking removes the tracking parameters of a TikTok link.
func StripTikTokTracking(rawURL string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return rawURL
	}
	query := parsed.Query()
	for key := range query {
		if tiktokTrackingParams.MatchString(key) {
			query.Del(key)
		}
	}
	parsed.RawQuery = query.Encode()
	return parsed.String()
}

// ExpandShortURL resolves the vm., vt. and vn. short links of TikTok.
func (s *TikTokService) ExpandShortURL(ctx context.Context, rawURL string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return rawURL
	}
	host := strings.ToLower(parsed.Hostname())
	switch host {
	case "vm.tiktok.com", "vt.tiktok.com", "vn.tiktok.com":
	default:
		return rawURL
	}

	requestCtx, cancel := context.WithTimeout(ctx, 4*time.Second)
	defer cancel()
	request, err := http.NewRequestWithContext(requestCtx, http.MethodHead, rawURL, nil)
	if err != nil {
		return rawURL
	}
	request.Header.Set("User-Agent", httpx.RandomUserAgent())
	client := &http.Client{Timeout: 4 * time.Second, CheckRedirect: func(req *http.Request, via []*http.Request) error {
		if len(via) >= 5 {
			return http.ErrUseLastResponse
		}
		return nil
	}}
	response, err := client.Do(request)
	if err != nil {
		return rawURL
	}
	defer response.Body.Close()
	if response.Request != nil && response.Request.URL != nil {
		return StripTikTokTracking(response.Request.URL.String())
	}
	return rawURL
}

// TikTokPayload is the normalised answer of the TikWM API.
type TikTokPayload struct {
	ID              string
	Title           string
	Cover           string
	Play            string
	WatermarkPlay   string
	HDPlay          string
	Music           string
	MusicPlay       string
	Author          string
	AuthorID        string
	Images          []string
	Duration        int
	PlayCount       int64
	DiggCount       int64
	CommentCount    int64
	ShareCount      int64
	SizeHD          int64
	Size            int64
	WatermarkSize   int64
	DownloadHeaders map[string]string
	WebpageURL      string
}

// FetchData loads the metadata of a TikTok post.
func (s *TikTokService) FetchData(ctx context.Context, videoURL string) (*TikTokPayload, error) {
	endpoint, err := url.Parse(tikwmAPIURL)
	if err != nil {
		return nil, fmt.Errorf("parse tikwm endpoint: %w", err)
	}
	query := endpoint.Query()
	query.Set("url", videoURL)
	query.Set("count", "12")
	query.Set("cursor", "0")
	query.Set("web", "1")
	query.Set("hd", "1")
	endpoint.RawQuery = query.Encode()

	requestCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	request, err := http.NewRequestWithContext(requestCtx, http.MethodGet, endpoint.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("build tikwm request: %w", err)
	}
	request.Header.Set("User-Agent", httpx.RandomUserAgent())
	request.Header.Set("Accept", "application/json")

	response, err := s.client.Do(request)
	if err != nil {
		return nil, fmt.Errorf("tikwm request failed: %w", err)
	}
	defer response.Body.Close()

	raw, err := io.ReadAll(io.LimitReader(response.Body, 4<<20))
	if err != nil {
		return nil, fmt.Errorf("read tikwm response: %w", err)
	}
	var envelope map[string]any
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return nil, fmt.Errorf("decode tikwm response: %w", err)
	}

	code := intValue(envelope["code"])
	if code != 0 {
		message := stringValue(envelope["msg"])
		if message == "" {
			message = stringValue(envelope["message"])
		}
		return nil, fmt.Errorf("tikwm error %d: %s", code, message)
	}

	data, _ := envelope["data"].(map[string]any)
	if data == nil {
		return nil, fmt.Errorf("tikwm response without data")
	}
	return normalizeTikwmData(data, videoURL), nil
}

func normalizeTikwmData(data map[string]any, videoURL string) *TikTokPayload {
	payload := &TikTokPayload{
		ID:            stringValue(data["id"]),
		Title:         stringValue(data["title"]),
		Cover:         normalizeTikwmURL(stringValue(data["cover"])),
		Play:          normalizeTikwmURL(stringValue(data["play"])),
		WatermarkPlay: normalizeTikwmURL(stringValue(data["wmplay"])),
		HDPlay:        normalizeTikwmURL(stringValue(data["hdplay"])),
		Music:         normalizeTikwmURL(stringValue(data["music"])),
		Duration:      intValue(data["duration"]),
		PlayCount:     int64Value(data["play_count"]),
		DiggCount:     int64Value(data["digg_count"]),
		CommentCount:  int64Value(data["comment_count"]),
		ShareCount:    int64Value(data["share_count"]),
		SizeHD:        int64Value(data["size_hd"]),
		Size:          int64Value(data["size"]),
		WatermarkSize: int64Value(data["wm_size"]),
		WebpageURL:    videoURL,
	}

	if musicInfo, ok := data["music_info"].(map[string]any); ok {
		payload.MusicPlay = normalizeTikwmURL(stringValue(musicInfo["play"]))
	}
	if author, ok := data["author"].(map[string]any); ok {
		payload.Author = stringValue(author["unique_id"])
		payload.AuthorID = stringValue(author["unique_id"])
		if payload.Title == "" {
			payload.Title = stringValue(author["nickname"])
		}
	}
	if images, ok := data["images"].([]any); ok {
		for _, item := range images {
			if text, ok := item.(string); ok {
				payload.Images = append(payload.Images, normalizeTikwmURL(text))
			}
		}
	}
	if payload.ID == "" {
		payload.ID = lastPathSegment(videoURL)
	}
	return payload
}

func normalizeTikwmURL(value string) string {
	if value == "" {
		return ""
	}
	if strings.HasPrefix(value, "//") {
		return "https:" + value
	}
	if strings.HasPrefix(value, "/") {
		return "https://tikwm.com" + value
	}
	return value
}

func lastPathSegment(rawURL string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}
	segments := strings.Split(strings.Trim(parsed.Path, "/"), "/")
	if len(segments) == 0 {
		return ""
	}
	return segments[len(segments)-1]
}

// VideoInfo converts the payload into the shared video metadata.
func (p *TikTokPayload) VideoInfo() *VideoInfo {
	size := p.SizeHD
	if size == 0 {
		size = p.Size
	}
	if size == 0 {
		size = p.WatermarkSize
	}
	return &VideoInfo{
		ID:              p.ID,
		Description:     p.Title,
		Author:          p.Author,
		Cover:           p.Cover,
		Views:           p.PlayCount,
		Likes:           p.DiggCount,
		Comments:        p.CommentCount,
		Shares:          p.ShareCount,
		MusicPlayURL:    p.MusicPlay,
		DurationSeconds: p.Duration,
		SizeHint:        size,
	}
}

// Post converts the payload into a post object.
func (p *TikTokPayload) Post() *Post {
	post := &Post{
		ID:          p.ID,
		Service:     ServiceTikTok,
		Author:      p.Author,
		Description: p.Title,
		Info:        p.VideoInfo(),
		Metadata: &Metadata{
			Title:     p.Title,
			Artists:   p.Author,
			Thumbnail: p.Cover,
			SourceURL: p.WebpageURL,
			URL:       p.WebpageURL,
		},
	}

	if len(p.Images) > 0 {
		for _, image := range p.Images {
			post.Media = append(post.Media, Media{URL: image, Type: KindPhoto, Thumb: p.Cover})
		}
		post.Album = len(post.Media) > 1
		return post
	}

	for _, candidate := range []string{p.Play, p.HDPlay, p.WatermarkPlay} {
		if candidate == "" {
			continue
		}
		post.Media = append(post.Media, Media{URL: candidate, Type: KindVideo, Thumb: p.Cover})
		break
	}
	if len(post.Media) == 0 && p.ID != "" {
		post.Media = append(post.Media, Media{
			URL:   fmt.Sprintf("https://tikwm.com/video/media/play/%s.mp4", p.ID),
			Type:  KindVideo,
			Thumb: p.Cover,
		})
	}
	return post
}

// FetchProfile loads the statistics of a TikTok profile.
func (s *TikTokService) FetchProfile(ctx context.Context, username string) (*UserInfo, error) {
	username = strings.TrimPrefix(username, "@")
	if username == "" {
		return nil, fmt.Errorf("empty tiktok username")
	}

	var secUID string
	var nickname string
	for attempt := 0; attempt < 10; attempt++ {
		requestCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
		request, err := http.NewRequestWithContext(requestCtx, http.MethodGet, "https://countik.com/api/exist/"+url.PathEscape(username), nil)
		if err != nil {
			cancel()
			return nil, fmt.Errorf("build countik request: %w", err)
		}
		request.Header.Set("User-Agent", httpx.RandomUserAgent())
		response, err := s.client.Do(request)
		if err != nil {
			cancel()
			return nil, fmt.Errorf("countik request failed: %w", err)
		}
		raw, _ := io.ReadAll(io.LimitReader(response.Body, 1<<20))
		response.Body.Close()
		cancel()

		var parsed map[string]any
		if err := json.Unmarshal(raw, &parsed); err == nil {
			secUID = stringValue(parsed["sec_uid"])
			nickname = stringValue(parsed["nickname"])
		}
		if secUID != "" {
			break
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(1500 * time.Millisecond):
		}
	}
	if secUID == "" {
		return nil, fmt.Errorf("could not resolve the tiktok profile of @%s", username)
	}

	requestCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	endpoint := "https://countik.com/api/userinfo?sec_user_id=" + url.QueryEscape(secUID)
	request, err := http.NewRequestWithContext(requestCtx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("build countik userinfo request: %w", err)
	}
	request.Header.Set("User-Agent", httpx.RandomUserAgent())
	response, err := s.client.Do(request)
	if err != nil {
		return nil, fmt.Errorf("countik userinfo failed: %w", err)
	}
	defer response.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(response.Body, 1<<20))

	var parsed map[string]any
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return nil, fmt.Errorf("decode countik userinfo: %w", err)
	}

	return &UserInfo{
		Nickname:    firstNonEmpty(nickname, username),
		Followers:   int64Value(parsed["followerCount"]),
		Videos:      int64Value(parsed["videoCount"]),
		Likes:       int64Value(parsed["heartCount"]),
		ProfilePic:  stringValue(parsed["avatarThumb"]),
		Description: stringValue(parsed["signature"]),
	}, nil
}
