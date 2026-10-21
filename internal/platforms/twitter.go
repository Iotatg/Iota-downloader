package platforms

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/Iotatg/iota-downloader/internal/httpx"
)

// TwitterService resolves X and Twitter posts through a public api.
type TwitterService struct {
	client *http.Client
	log    Logger
}

// NewTwitterService builds the Twitter resolver.
func NewTwitterService(log Logger) *TwitterService {
	return &TwitterService{client: httpx.NewClient(15*time.Second, 32), log: log}
}

// twitterEndpoints are the public apis that resolve a tweet.
//
// The first api that answers wins, the others are the fallback for a host
// that is blocked or under a challenge.
var twitterEndpoints = []string{
	"https://api.fxtwitter.com/2/status/",
	"https://api.vxtwitter.com/Twitter/status/",
}

var twitterStatusID = regexp.MustCompile(`/status(?:es)?/(\d+)`)

// TwitterID extracts the identifier of a tweet from a link.
func TwitterID(rawURL string) string {
	match := twitterStatusID.FindStringSubmatch(rawURL)
	if len(match) > 1 {
		return match[1]
	}
	return ""
}

// TwitterPost is the resolved content of a tweet.
type TwitterPost struct {
	ID         string
	Text       string
	Author     string
	AuthorName string
	URL        string
	Media      []Media
	MediaKind  string
}

// FetchPost loads a tweet and its media.
func (s *TwitterService) FetchPost(ctx context.Context, rawURL string) (*TwitterPost, error) {
	id := TwitterID(rawURL)
	if id == "" {
		return nil, fmt.Errorf("could not read the tweet id")
	}

	var errs []string
	for _, endpoint := range twitterEndpoints {
		tweet, err := s.fetchStatus(ctx, endpoint+id)
		if err != nil {
			errs = append(errs, err.Error())
			continue
		}
		post := ParseTwitterPost(tweet, rawURL, id)
		if post == nil {
			errs = append(errs, "the tweet holds no media")
			continue
		}
		return post, nil
	}
	return nil, fmt.Errorf("twitter: %s", strings.Join(errs, "; "))
}

// fetchStatus loads a tweet from one api and answers the tweet object.
func (s *TwitterService) fetchStatus(ctx context.Context, endpoint string) (map[string]any, error) {
	requestCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	request, err := http.NewRequestWithContext(requestCtx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("build the twitter request: %w", err)
	}
	request.Header.Set("User-Agent", httpx.RandomUserAgent())
	request.Header.Set("Accept", "application/json")

	response, err := s.client.Do(request)
	if err != nil {
		return nil, fmt.Errorf("the twitter request failed: %w", err)
	}
	defer response.Body.Close()

	raw, err := io.ReadAll(io.LimitReader(response.Body, 8<<20))
	if err != nil {
		return nil, fmt.Errorf("read the twitter answer: %w", err)
	}
	if response.StatusCode >= 400 {
		return nil, fmt.Errorf("the twitter api answered %d", response.StatusCode)
	}

	var parsed map[string]any
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return nil, fmt.Errorf("decode the twitter answer: %w", err)
	}
	if code := intValue(parsed["code"]); code != 0 && code != 200 {
		return nil, fmt.Errorf("the twitter api returned code %d", code)
	}

	// The api nests the tweet, the older one answers it flat.
	for _, key := range []string{"status", "tweet"} {
		if nested, ok := parsed[key].(map[string]any); ok {
			return nested, nil
		}
	}
	return parsed, nil
}

// ParseTwitterPost reads a tweet out of an api answer.
func ParseTwitterPost(tweet map[string]any, rawURL, id string) *TwitterPost {
	post := &TwitterPost{
		ID:   firstNonEmpty(stringValue(tweet["id"]), id),
		Text: firstNonEmpty(stringValue(tweet["text"]), stringValue(tweet["full_text"])),
		URL:  firstNonEmpty(stringValue(tweet["url"]), stringValue(tweet["tweetURL"]), rawURL),
	}

	if author, ok := tweet["author"].(map[string]any); ok {
		post.AuthorName = firstNonEmpty(stringValue(author["screen_name"]), stringValue(author["username"]))
		post.Author = firstNonEmpty(stringValue(author["name"]), post.AuthorName)
	}
	if post.Author == "" {
		post.Author = firstNonEmpty(stringValue(tweet["user_name"]), post.AuthorName)
	}
	if post.AuthorName == "" {
		post.AuthorName = firstNonEmpty(stringValue(tweet["user_screen_name"]), post.Author)
	}

	media, kind := ExtractTwitterMedia(tweet)
	post.Media = media
	post.MediaKind = kind
	return post
}

// ExtractTwitterMedia pulls the media entries out of a tweet answer.
func ExtractTwitterMedia(parsed map[string]any) ([]Media, string) {
	container := twitterMediaContainer(parsed)

	kind := ""
	var media []Media
	for index, item := range container {
		entry, ok := item.(map[string]any)
		if !ok {
			continue
		}
		mediaURL := firstNonEmpty(
			stringValue(entry["url"]),
			stringValue(entry["media_url"]),
			stringValue(entry["mediaUrl"]),
			stringValue(entry["image"]),
			stringValue(entry["image_url"]),
			stringValue(entry["imageUrl"]),
			stringValue(entry["video_url"]),
			stringValue(entry["videoUrl"]),
			stringValue(entry["src"]),
		)
		if mediaURL == "" {
			continue
		}
		itemKind := NormalizeTwitterMediaKind(stringValue(entry["type"]))
		if itemKind == "" {
			itemKind = InferTwitterMediaKind(mediaURL)
		}
		if itemKind == "" {
			itemKind = KindVideo
		}
		if kind == "" {
			kind = itemKind
		}
		thumb := firstNonEmpty(
			stringValue(entry["thumb"]),
			stringValue(entry["thumbnail"]),
			stringValue(entry["thumbnail_url"]),
			stringValue(entry["thumbnailUrl"]),
			stringValue(entry["poster"]),
			stringValue(entry["poster_url"]),
			stringValue(entry["preview"]),
			stringValue(entry["preview_url"]),
		)
		if thumb == "" && itemKind == KindVideo {
			thumb = mediaURL
		}
		_ = index
		media = append(media, Media{URL: mediaURL, Type: itemKind, Thumb: thumb})
	}
	return media, kind
}

// twitterMediaContainer finds the list of media entries of a tweet answer.
func twitterMediaContainer(parsed map[string]any) []any {
	if media, ok := parsed["media"].(map[string]any); ok {
		if list, ok := media["all"].([]any); ok {
			return list
		}
	}
	if list, ok := parsed["media_extended"].([]any); ok {
		return list
	}
	for _, key := range []string{"mediaURLs", "media_urls", "images", "videos", "videoURLs", "video_urls"} {
		if list, ok := parsed[key].([]any); ok {
			return list
		}
	}
	return nil
}

// NormalizeTwitterMediaKind maps the different kind names onto the shared
// media kinds.
func NormalizeTwitterMediaKind(value string) string {
	switch strings.ToLower(value) {
	case "image", "photo":
		return KindPhoto
	case "video", "gif":
		return KindVideo
	default:
		return ""
	}
}

// InferTwitterMediaKind guesses the kind of a media file from its extension.
func InferTwitterMediaKind(mediaURL string) string {
	lower := strings.ToLower(mediaURL)
	switch {
	case strings.Contains(lower, ".jpg"), strings.Contains(lower, ".jpeg"),
		strings.Contains(lower, ".png"), strings.Contains(lower, ".webp"),
		strings.Contains(lower, ".gif"):
		return KindPhoto
	case strings.Contains(lower, ".mp4"), strings.Contains(lower, ".mov"),
		strings.Contains(lower, ".m4v"), strings.Contains(lower, ".webm"):
		return KindVideo
	default:
		return ""
	}
}
