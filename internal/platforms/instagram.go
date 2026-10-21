package platforms

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/Iotatg/iota-downloader/internal/httpx"
)

// InstagramService resolves Instagram posts and reels.
//
// Instagram asks every anonymous visitor to log in, so the resolver works with
// a session cookie. Three backends are tried in order:
//
//  1. the private media endpoint with a session identifier,
//  2. yt-dlp with an exported cookie file,
//  3. a Cobalt backend when one is configured.
type InstagramService struct {
	SessionID   string
	CookiesFile string
	AppID       string
	Cobalt      *CobaltClient
	YTDLP       *YTDLP
	HTTP        *http.Client
	Log         Logger
}

// NewInstagramService builds the Instagram resolver.
func NewInstagramService(cobalt *CobaltClient, log Logger) *InstagramService {
	return &InstagramService{
		AppID:  "936619743392459",
		Cobalt: cobalt,
		HTTP:   httpx.NewClient(20*time.Second, 16),
		Log:    log,
	}
}

// ErrInstagramSessionMissing reports that Instagram needs a logged in session
// before it answers a request.
var ErrInstagramSessionMissing = errors.New("instagram needs a session cookie")

var instagramPostID = regexp.MustCompile(`instagram\.com/(?:p|reels|reel|tv)/([^/?#&]+)`)

// instagramShortcode extracts the shortcode of an Instagram link.
func instagramShortcode(rawURL string) string {
	match := instagramPostID.FindStringSubmatch(rawURL)
	if len(match) > 1 {
		return match[1]
	}
	return ""
}

// FetchPost resolves an Instagram post into a post object.
func (s *InstagramService) FetchPost(ctx context.Context, rawURL string) (*Post, error) {
	shortcode := instagramShortcode(rawURL)
	if shortcode == "" {
		return nil, fmt.Errorf("instagram link without a shortcode")
	}

	var errs []string
	if s.SessionID != "" || s.CookiesFile != "" {
		post, err := s.fetchWithSession(ctx, rawURL, shortcode)
		if err == nil {
			return post, nil
		}
		errs = append(errs, err.Error())
	}

	if s.CookiesFile != "" && s.YTDLP != nil {
		post, err := s.fetchWithYTDLP(ctx, rawURL)
		if err == nil {
			return post, nil
		}
		errs = append(errs, err.Error())
	}

	if s.Cobalt != nil && s.Cobalt.BaseURL != "" && s.Cobalt.APIKey != "" {
		post, err := s.fetchWithCobalt(ctx, rawURL, shortcode)
		if err == nil {
			return post, nil
		}
		errs = append(errs, err.Error())
	}

	if len(errs) == 0 {
		return nil, ErrInstagramSessionMissing
	}
	return nil, fmt.Errorf("instagram: %s", strings.Join(errs, "; "))
}

// instagramMedia is one entry of the media answer of Instagram.
type instagramMedia struct {
	ID            string `json:"pk"`
	MediaType     int    `json:"media_type"`
	Code          string `json:"code"`
	VideoVersions []struct {
		Type int    `json:"type"`
		URL  string `json:"url"`
	} `json:"video_versions"`
	ImageVersions *struct {
		Candidates []struct {
			URL string `json:"url"`
		} `json:"candidates"`
	} `json:"image_versions2"`
	ThumbnailURL string `json:"thumbnail_url"`
	DisplayURL   string `json:"display_url"`
	PlayCount    int64  `json:"play_count"`
	LikeCount    int64  `json:"like_count"`
	CommentCount int64  `json:"comment_count"`
	TakenAt      int64  `json:"taken_at"`
	HasAudio     bool   `json:"has_audio"`
	Caption      struct {
		Text string `json:"text"`
	} `json:"caption"`
	Owner struct {
		Username      string `json:"username"`
		FullName      string `json:"full_name"`
		ProfilePicURL string `json:"profile_pic_url_hd"`
	} `json:"owner"`
	CarouselMedia []instagramMedia `json:"carousel_media"`
}

// instagramInfo is the answer of the private media endpoint.
type instagramInfo struct {
	Items               []instagramMedia `json:"items"`
	NumResults          int              `json:"num_results"`
	Status              string           `json:"status"`
	Message             string           `json:"message"`
	LoginRequired       bool             `json:"login_required"`
	RequireLogin        bool             `json:"require_login"`
	MediaIDNotFound     bool             `json:"media_id_not_found"`
	MediaIDNotAvailable bool             `json:"media_id_not_available"`
}

// fetchWithSession reads the post through the private Instagram endpoint.
func (s *InstagramService) fetchWithSession(ctx context.Context, rawURL, shortcode string) (*Post, error) {
	mediaID := instagramMediaID(shortcode)
	if mediaID == "" {
		return nil, fmt.Errorf("shortcode %q cannot be converted into a media id", shortcode)
	}

	endpoint := "https://i.instagram.com/api/v1/media/" + mediaID + "/info/"

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("build instagram request: %w", err)
	}
	appID := s.AppID
	if appID == "" {
		appID = "936619743392459"
	}
	request.Header.Set("x-ig-app-id", appID)
	request.Header.Set("User-Agent", instagramAppUserAgent)
	request.Header.Set("Accept", "*/*")
	request.Header.Set("Accept-Language", "en-US,en;q=0.9")
	if cookie := s.cookieHeader(); cookie != "" {
		request.Header.Set("Cookie", cookie)
	}

	response, err := s.client().Do(request)
	if err != nil {
		return nil, fmt.Errorf("instagram request failed: %w", err)
	}
	defer response.Body.Close()

	raw, err := io.ReadAll(io.LimitReader(response.Body, 16<<20))
	if err != nil {
		return nil, fmt.Errorf("read instagram answer: %w", err)
	}

	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("instagram answered %d: %s", response.StatusCode, instagramErrorText(raw))
	}

	var info instagramInfo
	if err := json.Unmarshal(raw, &info); err != nil {
		return nil, fmt.Errorf("decode instagram answer: %w", err)
	}
	if len(info.Items) == 0 {
		message := info.Message
		if message == "" {
			message = "no media"
		}
		return nil, fmt.Errorf("instagram returned no media: %s", message)
	}

	return instagramPost(rawURL, shortcode, info.Items)
}

// instagramPost converts the media entries of Instagram into a post.
func instagramPost(rawURL, shortcode string, items []instagramMedia) (*Post, error) {
	item := items[0]
	post := &Post{
		ID:      shortcode,
		Service: ServiceInstagram,
		Author:  item.Owner.Username,
	}

	post.Metadata = &Metadata{
		Title:     strings.TrimSpace(item.Caption.Text),
		Artists:   item.Owner.FullName,
		Thumbnail: item.ThumbnailURL,
		SourceURL: rawURL,
		URL:       rawURL,
		Duration:  0,
	}
	if post.Author == "" {
		post.Author = "instagram_user"
	}
	if post.Metadata.Artists == "" {
		post.Metadata.Artists = post.Author
	}
	if post.Metadata.Title == "" {
		post.Metadata.Title = "Instagram " + shortcode
	}
	post.Description = post.Metadata.Title

	post.Info = &VideoInfo{
		ID:       shortcode,
		Author:   post.Author,
		Cover:    item.ThumbnailURL,
		Views:    item.PlayCount,
		Likes:    item.LikeCount,
		Comments: item.CommentCount,
	}

	for _, entry := range items {
		kind := KindVideo
		url := ""
		if len(entry.VideoVersions) > 0 {
			url = entry.VideoVersions[0].URL
		} else if entry.ImageVersions != nil && len(entry.ImageVersions.Candidates) > 0 {
			url = entry.ImageVersions.Candidates[0].URL
			kind = KindPhoto
		}
		if url == "" {
			continue
		}
		post.Media = append(post.Media, Media{URL: url, Type: kind, Thumb: entry.ThumbnailURL})
	}
	if len(post.Media) == 0 {
		return nil, fmt.Errorf("instagram returned no downloadable file")
	}
	post.Album = len(post.Media) > 1
	return post, nil
}

// fetchWithYTDLP resolves the post through yt-dlp with the exported cookies.
func (s *InstagramService) fetchWithYTDLP(ctx context.Context, rawURL string) (*Post, error) {
	if s.YTDLP == nil {
		return nil, fmt.Errorf("yt-dlp is not available")
	}
	info, err := s.YTDLP.ExtractInfoWithCookies(ctx, rawURL, s.CookiesFile)
	if err != nil {
		return nil, err
	}

	shortcode := instagramShortcode(rawURL)
	if shortcode == "" {
		shortcode = hashKey(rawURL)
	}

	post := &Post{
		ID:      shortcode,
		Service: ServiceInstagram,
		Author:  stringValue(info["uploader"]),
	}
	if post.Author == "" {
		post.Author = "instagram_user"
	}

	title := stringValue(info["title"])
	if title == "" {
		title = stringValue(info["description"])
	}
	thumbnail := stringValue(info["thumbnail"])
	post.Metadata = &Metadata{
		Title:     title,
		Artists:   stringValue(info["uploader"]),
		Thumbnail: thumbnail,
		SourceURL: rawURL,
		URL:       rawURL,
	}
	if post.Metadata.Artists == "" {
		post.Metadata.Artists = post.Author
	}
	if post.Metadata.Title == "" {
		post.Metadata.Title = "Instagram " + shortcode
	}
	post.Description = post.Metadata.Title

	post.Info = &VideoInfo{
		ID:     shortcode,
		Author: post.Author,
		Cover:  thumbnail,
		Views:  int64Value(info["view_count"]),
		Likes:  int64Value(info["like_count"]),
	}

	formats, _ := info["formats"].([]any)
	for _, entry := range formats {
		format, ok := entry.(map[string]any)
		if !ok {
			continue
		}
		url, _ := format["url"].(string)
		if url == "" {
			continue
		}
		kind := KindVideo
		if format["vcodec"] == "none" && format["acodec"] != "none" {
			kind = KindAudio
		}
		if format["ext"] == "mp4" || format["ext"] == "webm" {
			kind = KindVideo
		}
		post.Media = append(post.Media, Media{URL: url, Type: kind, Thumb: thumbnail})
	}
	if len(post.Media) == 0 {
		return nil, fmt.Errorf("yt-dlp returned no downloadable file")
	}
	post.Album = len(post.Media) > 1
	return post, nil
}

// fetchWithCobalt resolves the post through a Cobalt backend.
func (s *InstagramService) fetchWithCobalt(ctx context.Context, rawURL, shortcode string) (*Post, error) {
	payload := map[string]any{
		"url":             rawURL,
		"videoQuality":    "720",
		"downloadMode":    "auto",
		"alwaysProxy":     true,
		"localProcessing": "disabled",
	}

	response, err := s.Cobalt.Fetch(ctx, payload, ServiceInstagram)
	if err != nil {
		return nil, err
	}
	items, err := ParseItems(response, false, false)
	if err != nil {
		return nil, err
	}

	post := &Post{
		ID:      shortcode,
		Service: ServiceInstagram,
	}
	if post.ID == "" {
		post.ID = hashKey(rawURL)
	}

	metadata := decodeMetadata(response)
	post.Description = firstNonEmpty(
		metadataString(metadata, "description"),
		metadataString(metadata, "caption"),
		metadataString(metadata, "title"),
		metadataString(metadata, "text"),
		response.Filename,
	)
	post.Author = firstNonEmpty(
		metadataString(metadata, "author"),
		metadataString(metadata, "uploader"),
		metadataString(metadata, "username"),
		metadataString(metadata, "creator"),
		authorFromFilename(response.Filename),
		"instagram_user",
	)

	for _, item := range items {
		post.Media = append(post.Media, Media{URL: item.URL, Type: item.Type, Thumb: item.Thumb})
	}
	post.Album = len(post.Media) > 1
	return post, nil
}

// cookieHeader builds the cookie header of the Instagram request.
func (s *InstagramService) cookieHeader() string {
	parts := make([]string, 0, 3)
	if s.SessionID != "" {
		parts = append(parts, "sessionid="+s.SessionID)
	}
	if s.CookiesFile != "" {
		if csrftoken := readCookieValue(s.CookiesFile, "csrftoken"); csrftoken != "" {
			parts = append(parts, "csrftoken="+csrftoken)
		}
		if sessionid := readCookieValue(s.CookiesFile, "sessionid"); sessionid != "" && s.SessionID == "" {
			parts = append(parts, "sessionid="+sessionid)
		}
	}
	return strings.Join(parts, "; ")
}

// readCookieValue reads a single value of a Netscape cookie file.
func readCookieValue(path, name string) string {
	raw, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(raw), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fields := strings.Split(line, "\t")
		if len(fields) < 7 {
			continue
		}
		if fields[5] == name {
			return fields[6]
		}
	}
	return ""
}

// client returns the HTTP client of the service.
func (s *InstagramService) client() *http.Client {
	if s.HTTP != nil {
		return s.HTTP
	}
	return http.DefaultClient
}

// instagramMediaID converts a shortcode into the numeric media identifier.
func instagramMediaID(shortcode string) string {
	const alphabet = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789-_"
	var id uint64
	for _, char := range shortcode {
		index := strings.IndexRune(alphabet, char)
		if index < 0 {
			return ""
		}
		id = id*64 + uint64(index)
	}
	return strconv.FormatUint(id, 10)
}

// instagramErrorText extracts a short error message of an API answer.
func instagramErrorText(raw []byte) string {
	var parsed struct {
		Message string `json:"message"`
	}
	if err := json.Unmarshal(raw, &parsed); err == nil && parsed.Message != "" {
		return parsed.Message
	}
	trimmed := strings.TrimSpace(string(raw))
	if len(trimmed) > 160 {
		trimmed = trimmed[:160]
	}
	return trimmed
}

// instagramAppUserAgent is the user agent of the Instagram mobile application.
const instagramAppUserAgent = "Instagram 219.0.0.12.117 Android (29/10; 420dpi; 1080x2152; OnePlus; ONEPLUS A6013; OnePlus6T; qcom; en_US; 458229258)"
