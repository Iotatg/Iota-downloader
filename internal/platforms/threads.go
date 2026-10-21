package platforms

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/Iotatg/iota-downloader/internal/httpx"
)

// ThreadsService resolves Threads posts from the public post metadata.
//
// Threads belongs to the same platform family as Instagram and asks every
// anonymous visitor to log in, the session cookie of a logged in Instagram
// session opens the post as well.
type ThreadsService struct {
	SessionID string
	client    *http.Client
	log       Logger
}

// NewThreadsService builds the Threads resolver.
//
// The session id is the cookie of a logged in Instagram session, it is
// optional because a public post can be read without it.
func NewThreadsService(log Logger, sessionID string) *ThreadsService {
	return &ThreadsService{SessionID: sessionID, client: httpx.NewClient(20*time.Second, 16), log: log}
}

// ErrThreadsSessionMissing reports that Threads needs a logged in session
// before it answers a request.
var ErrThreadsSessionMissing = errors.New("threads needs a session cookie")

var threadsPostURL = regexp.MustCompile(`^https?://(?:www\.)?threads\.(?:com|net)/@([A-Za-z0-9._-]+)/post/([A-Za-z0-9_-]+)/*$`)

var threadsScriptBlock = regexp.MustCompile(`(?s)<script[^>]*type="application/json"[^>]*>(.*?)</script>`)

const threadsUserAgent = "Mozilla/5.0 (compatible; Googlebot/2.1; +http://www.google.com/bot.html)"

// ThreadsPostCode extracts the username and the post code of a Threads link.
func ThreadsPostCode(rawURL string) (string, string, bool) {
	match := threadsPostURL.FindStringSubmatch(strings.TrimSpace(rawURL))
	if len(match) < 3 {
		return "", "", false
	}
	return match[1], match[2], true
}

// CanonicalThreadsURL builds the canonical form of a Threads link.
func CanonicalThreadsURL(rawURL string) string {
	username, code, ok := ThreadsPostCode(rawURL)
	if !ok {
		return rawURL
	}
	return "https://www.threads.com/@" + username + "/post/" + code
}

// FetchPostHTML loads the HTML of a Threads post.
func (s *ThreadsService) FetchPostHTML(ctx context.Context, rawURL string) (string, error) {
	page, _, err := s.fetchPage(ctx, rawURL)
	return page, err
}

// fetchPage loads the page of a post and answers the final url of the
// request.
//
// The platform sends the visitor to the login page when a post needs a
// session and to an error page when the post does not exist, both answers are
// read from the final url.
func (s *ThreadsService) fetchPage(ctx context.Context, rawURL string) (string, string, error) {
	requestCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()

	request, err := http.NewRequestWithContext(requestCtx, http.MethodGet, CanonicalThreadsURL(rawURL), nil)
	if err != nil {
		return "", "", fmt.Errorf("build threads request: %w", err)
	}
	request.Header.Set("User-Agent", threadsUserAgent)
	request.Header.Set("Accept", "text/html,application/xhtml+xml")
	request.Header.Set("Accept-Language", "en-US,en;q=0.9")
	if s.SessionID != "" {
		request.Header.Set("Cookie", "sessionid="+s.SessionID)
	}

	response, err := s.client.Do(request)
	if err != nil {
		return "", "", fmt.Errorf("threads request failed: %w", err)
	}
	defer response.Body.Close()

	finalURL := ""
	finalPath := ""
	if response.Request != nil && response.Request.URL != nil {
		finalURL = response.Request.URL.String()
		finalPath = response.Request.URL.Path
	}

	// A post that asks for a login is answered with the login page, the
	// media of the post is not part of it.
	if strings.Contains(finalPath, "/login") {
		return "", finalURL, ErrThreadsSessionMissing
	}

	if response.StatusCode >= 400 {
		return "", finalURL, fmt.Errorf("threads returned %d", response.StatusCode)
	}

	raw, err := io.ReadAll(io.LimitReader(response.Body, 16<<20))
	if err != nil {
		return "", finalURL, fmt.Errorf("read threads response: %w", err)
	}
	return string(raw), finalURL, nil
}

// ParseThreadsPost extracts the media of a Threads post from its HTML.
func ParseThreadsPost(page, postCode string) *Post {
	needle := `"code":"` + postCode + `"`
	if !strings.Contains(page, needle) {
		return nil
	}

	for _, match := range threadsScriptBlock.FindAllStringSubmatch(page, -1) {
		if len(match) < 2 {
			continue
		}
		var payload any
		if err := json.Unmarshal([]byte(match[1]), &payload); err != nil {
			continue
		}
		node := findNodeByCode(payload, postCode)
		if node == nil {
			continue
		}
		return buildThreadsPost(node, postCode)
	}
	return nil
}

func findNodeByCode(payload any, postCode string) map[string]any {
	switch typed := payload.(type) {
	case map[string]any:
		if code, ok := typed["code"].(string); ok && code == postCode {
			return typed
		}
		for _, value := range typed {
			if found := findNodeByCode(value, postCode); found != nil {
				return found
			}
		}
	case []any:
		for _, value := range typed {
			if found := findNodeByCode(value, postCode); found != nil {
				return found
			}
		}
	}
	return nil
}

func buildThreadsPost(node map[string]any, postCode string) *Post {
	post := &Post{
		ID:      postCode,
		Service: ServiceThreads,
	}

	if caption, ok := node["caption"].(map[string]any); ok {
		post.Description = stringValue(caption["text"])
	}
	if user, ok := node["user"].(map[string]any); ok {
		post.Author = firstNonEmpty(stringValue(user["username"]), "threads")
	}
	if post.Author == "" {
		post.Author = "threads"
	}

	source := threadsMediaSource(node)
	media := threadsMediaList(source)
	for _, item := range media {
		post.Media = append(post.Media, item)
	}
	post.Album = len(post.Media) > 1

	if len(post.Media) == 0 && post.Description == "" {
		return nil
	}
	return post
}

func threadsMediaSource(node map[string]any) map[string]any {
	if _, ok := node["carousel_media"]; ok {
		return node
	}
	if _, ok := node["video_versions"]; ok {
		return node
	}
	if _, ok := node["image_versions2"]; ok {
		return node
	}
	if info, ok := node["text_post_app_info"].(map[string]any); ok {
		if linked, ok := info["linked_inline_media"].(map[string]any); ok {
			return linked
		}
		if share, ok := info["share_info"].(map[string]any); ok {
			if quoted, ok := share["quoted_attachment_post"].(map[string]any); ok {
				return quoted
			}
		}
	}
	return node
}

func threadsMediaList(source map[string]any) []Media {
	var media []Media

	if versions, ok := source["video_versions"].([]any); ok {
		best := bestThreadsVersion(versions)
		if best != nil {
			media = append(media, Media{URL: stringValue(best["url"]), Type: KindVideo, Thumb: threadsThumb(source)})
		}
	}
	if imageVersions, ok := source["image_versions2"].(map[string]any); ok {
		if candidates, ok := imageVersions["candidates"].([]any); ok {
			best := bestThreadsVersion(candidates)
			if best != nil {
				media = append(media, Media{URL: stringValue(best["url"]), Type: KindPhoto, Thumb: stringValue(best["url"])})
			}
		}
	}
	if carousel, ok := source["carousel_media"].([]any); ok {
		for _, item := range carousel {
			entry, ok := item.(map[string]any)
			if !ok {
				continue
			}
			if versions, ok := entry["video_versions"].([]any); ok {
				if best := bestThreadsVersion(versions); best != nil {
					media = append(media, Media{URL: stringValue(best["url"]), Type: KindVideo, Thumb: threadsThumb(entry)})
				}
				continue
			}
			if imageVersions, ok := entry["image_versions2"].(map[string]any); ok {
				if candidates, ok := imageVersions["candidates"].([]any); ok {
					if best := bestThreadsVersion(candidates); best != nil {
						media = append(media, Media{URL: stringValue(best["url"]), Type: KindPhoto, Thumb: stringValue(best["url"])})
					}
				}
			}
		}
	}
	return media
}

func threadsThumb(source map[string]any) string {
	if imageVersions, ok := source["image_versions2"].(map[string]any); ok {
		if candidates, ok := imageVersions["candidates"].([]any); ok {
			if best := bestThreadsVersion(candidates); best != nil {
				return stringValue(best["url"])
			}
		}
	}
	return ""
}

func bestThreadsVersion(candidates []any) map[string]any {
	var best map[string]any
	bestArea := 0
	for _, item := range candidates {
		entry, ok := item.(map[string]any)
		if !ok {
			continue
		}
		width := intValue(entry["width"])
		height := intValue(entry["height"])
		area := width * height
		if area > bestArea {
			bestArea = area
			best = entry
		}
	}
	return best
}

// FetchPost resolves a Threads post into a post object.
func (s *ThreadsService) FetchPost(ctx context.Context, rawURL string) (*Post, error) {
	_, code, ok := ThreadsPostCode(rawURL)
	if !ok {
		return nil, fmt.Errorf("unsupported threads link")
	}
	page, finalURL, err := s.fetchPage(ctx, rawURL)
	if err != nil {
		return nil, err
	}
	post := ParseThreadsPost(page, code)
	if post == nil {
		// The platform answers a post that does not exist with its error
		// page, the visitor is sent to the login page when a session is
		// missing.
		if strings.Contains(finalURL, "invalid_post") {
			return nil, fmt.Errorf("the threads post does not exist")
		}
		return nil, fmt.Errorf("could not read the threads post")
	}
	return post, nil
}
