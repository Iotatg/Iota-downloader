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

// MastodonService resolves Mastodon posts.
//
// A Mastodon post lives on the instance of its author. The public api of the
// instance serves the post together with its media, the open graph tags of
// the page are the fallback for the instances that ask for a session.
type MastodonService struct {
	client *http.Client
	log    Logger
}

// NewMastodonService builds the Mastodon resolver.
func NewMastodonService(log Logger) *MastodonService {
	return &MastodonService{client: httpx.NewClient(25*time.Second, 16), log: log}
}

// mastodonStatusID matches the status id at the end of a post link.
var mastodonStatusID = regexp.MustCompile(`/(\d+)/?$`)

// MastodonStatusID extracts the id of a Mastodon post.
func MastodonStatusID(rawURL string) (string, bool) {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return "", false
	}
	match := mastodonStatusID.FindStringSubmatch(parsed.Path)
	if len(match) < 2 {
		return "", false
	}
	return match[1], true
}

// MastodonAccount is the author of a Mastodon post.
type MastodonAccount struct {
	Acct           string `json:"acct"`
	DisplayName    string `json:"display_name"`
	Avatar         string `json:"avatar"`
	URL            string `json:"url"`
	Note           string `json:"note"`
	FollowersCount int    `json:"followers_count"`
}

// MastodonMedia is a single attachment of a Mastodon post.
type MastodonMedia struct {
	ID          string `json:"id"`
	Type        string `json:"type"`
	URL         string `json:"url"`
	PreviewURL  string `json:"preview_url"`
	RemoteURL   string `json:"remote_url"`
	MimeType    string `json:"mime_type"`
	Description string `json:"description"`
}

// MastodonStatus is the payload of a Mastodon status.
type MastodonStatus struct {
	ID        string          `json:"id"`
	URL       string          `json:"url"`
	Content   string          `json:"content"`
	CreatedAt string          `json:"created_at"`
	Account   MastodonAccount `json:"account"`
	Media     []MastodonMedia `json:"media_attachments"`
	Reblog    *MastodonStatus `json:"reblog"`
}

// mastodonHeaders returns the browser headers of a request.
//
// The compression is left to the transport of the http client, a body that
// was compressed by the server is then decoded by the client.
func mastodonHeaders() map[string]string {
	headers := httpx.BrowserHeaders()
	delete(headers, "Accept-Encoding")
	return headers
}

// FetchStatus reads a status from the public api of its instance.
func (s *MastodonService) FetchStatus(ctx context.Context, rawURL string) (*MastodonStatus, error) {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return nil, fmt.Errorf("parse mastodon link: %w", err)
	}
	id, ok := MastodonStatusID(rawURL)
	if !ok {
		return nil, fmt.Errorf("unsupported mastodon link")
	}

	endpoint := "https://" + parsed.Host + "/api/v1/statuses/" + id
	requestCtx, cancel := context.WithTimeout(ctx, 25*time.Second)
	defer cancel()

	request, err := http.NewRequestWithContext(requestCtx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("build mastodon request: %w", err)
	}
	for name, value := range mastodonHeaders() {
		request.Header.Set(name, value)
	}
	request.Header.Set("Accept", "application/json")

	response, err := s.client.Do(request)
	if err != nil {
		return nil, fmt.Errorf("mastodon request failed: %w", err)
	}
	defer response.Body.Close()

	raw, err := io.ReadAll(io.LimitReader(response.Body, 8<<20))
	if err != nil {
		return nil, fmt.Errorf("read mastodon response: %w", err)
	}
	if response.StatusCode >= 400 {
		return nil, fmt.Errorf("mastodon returned %d", response.StatusCode)
	}

	var status MastodonStatus
	if err := json.Unmarshal(raw, &status); err != nil {
		return nil, fmt.Errorf("decode mastodon status: %w", err)
	}
	return &status, nil
}

// FetchPage loads the HTML of a Mastodon post.
func (s *MastodonService) FetchPage(ctx context.Context, rawURL string) (string, error) {
	requestCtx, cancel := context.WithTimeout(ctx, 25*time.Second)
	defer cancel()

	request, err := http.NewRequestWithContext(requestCtx, http.MethodGet, strings.TrimSpace(rawURL), nil)
	if err != nil {
		return "", fmt.Errorf("build mastodon page request: %w", err)
	}
	for name, value := range mastodonHeaders() {
		request.Header.Set(name, value)
	}
	request.Header.Set("Accept", "text/html,application/xhtml+xml")

	response, err := s.client.Do(request)
	if err != nil {
		return "", fmt.Errorf("mastodon page request failed: %w", err)
	}
	defer response.Body.Close()

	if response.StatusCode >= 400 {
		return "", fmt.Errorf("mastodon returned %d", response.StatusCode)
	}

	raw, err := io.ReadAll(io.LimitReader(response.Body, 8<<20))
	if err != nil {
		return "", fmt.Errorf("read mastodon page: %w", err)
	}
	return string(raw), nil
}

// FetchPost resolves a Mastodon post into a post object.
//
// The public api of the instance is asked first, the page itself is the
// fallback for the instances that do not serve anonymous readers.
func (s *MastodonService) FetchPost(ctx context.Context, rawURL string) (*Post, error) {
	status, err := s.FetchStatus(ctx, rawURL)
	if err == nil {
		if post := PostFromMastodonStatus(status, rawURL); post != nil {
			return post, nil
		}
	} else if s.log != nil {
		s.log.Warning("mastodon api lookup failed, the page is used: %v", err)
	}

	page, pageErr := s.FetchPage(ctx, rawURL)
	if pageErr != nil {
		if err != nil {
			return nil, err
		}
		return nil, pageErr
	}

	post := PostFromMastodonPage(page, rawURL)
	if post == nil {
		return nil, fmt.Errorf("could not read the mastodon post")
	}
	return post, nil
}

// PostFromMastodonStatus converts a status into a post.
func PostFromMastodonStatus(status *MastodonStatus, rawURL string) *Post {
	if status == nil {
		return nil
	}
	if status.Reblog != nil {
		status = status.Reblog
	}

	post := &Post{
		ID:          status.ID,
		Service:     ServiceMastodon,
		Author:      firstNonEmptyString(status.Account.DisplayName, status.Account.Acct),
		Description: StripHTMLTags(status.Content),
		Metadata: &Metadata{
			Title:     firstNonEmptyString(StripHTMLTags(status.Content), status.Account.DisplayName, status.Account.Acct),
			Artists:   status.Account.Acct,
			Thumbnail: status.Account.Avatar,
			SourceURL: firstNonEmptyString(status.URL, rawURL),
			URL:       firstNonEmptyString(status.URL, rawURL),
		},
	}

	for _, item := range status.Media {
		media := Media{URL: firstNonEmptyString(item.URL, item.RemoteURL), Thumb: item.PreviewURL}
		switch item.Type {
		case "video", "gifv":
			media.Type = KindVideo
		case "audio":
			media.Type = KindAudio
		default:
			media.Type = KindPhoto
		}
		if media.URL == "" {
			continue
		}
		if post.Metadata.Thumbnail == "" {
			post.Metadata.Thumbnail = media.Thumb
		}
		post.Media = append(post.Media, media)
	}

	post.Album = len(post.Media) > 1
	if len(post.Media) == 0 && post.Description == "" {
		return nil
	}
	return post
}

// mastodonMetaTag matches a meta tag of a page.
var mastodonMetaTag = regexp.MustCompile(`(?is)<meta\s+([^>]*)>`)

// mastodonMetaAttribute matches one attribute of a meta tag.
var mastodonMetaAttribute = regexp.MustCompile(`(?is)([a-zA-Z_:][-a-zA-Z0-9_:.]*)\s*=\s*(?:"([^"]*)"|'([^']*)'|([^\s"'>]+))`)

// mastodonHTMLTag matches the markup of a text.
var mastodonHTMLTag = regexp.MustCompile(`(?s)<[^>]*>`)

// StripHTMLTags removes the html markup of a text.
func StripHTMLTags(value string) string {
	value = mastodonHTMLTag.ReplaceAllString(value, " ")
	value = strings.NewReplacer(
		"&amp;", "&",
		"&lt;", "<",
		"&gt;", ">",
		"&quot;", "\"",
		"&#39;", "'",
		"&nbsp;", " ",
	).Replace(value)
	return strings.Join(strings.Fields(value), " ")
}

// firstNonEmptyString returns the first value that is not empty.
func firstNonEmptyString(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

// MastodonMetaTags collects the meta tags of a page.
func MastodonMetaTags(page string) map[string]string {
	tags := make(map[string]string)
	for _, tag := range mastodonMetaTag.FindAllStringSubmatch(page, -1) {
		if len(tag) < 2 {
			continue
		}
		attributes := make(map[string]string)
		for _, attribute := range mastodonMetaAttribute.FindAllStringSubmatch(tag[1], -1) {
			if len(attribute) < 5 {
				continue
			}
			value := attribute[2]
			if value == "" {
				value = attribute[3]
			}
			if value == "" {
				value = attribute[4]
			}
			attributes[strings.ToLower(attribute[1])] = value
		}
		key := attributes["property"]
		if key == "" {
			key = attributes["name"]
		}
		if key == "" {
			continue
		}
		tags[strings.ToLower(key)] = attributes["content"]
	}
	return tags
}

// PostFromMastodonPage extracts the media of a Mastodon post from its HTML.
//
// The open graph tags of the page carry the media of the post, they are the
// fallback for the instances that do not serve the public api.
func PostFromMastodonPage(page, rawURL string) *Post {
	tags := MastodonMetaTags(page)
	if len(tags) == 0 {
		return nil
	}

	video := firstNonEmptyString(tags["og:video:secure_url"], tags["og:video:url"], tags["og:video"], tags["twitter:player:stream"])
	image := firstNonEmptyString(tags["og:image:secure_url"], tags["og:image:url"], tags["og:image"])
	audio := firstNonEmptyString(tags["og:audio:secure_url"], tags["og:audio:url"], tags["og:audio"])

	if video == "" && image == "" && audio == "" {
		return nil
	}

	title := tags["og:title"]
	post := &Post{
		ID:          rawURL,
		Service:     ServiceMastodon,
		Author:      firstNonEmptyString(tags["author"], tags["og:site_name"]),
		Description: StripHTMLTags(tags["og:description"]),
		Metadata: &Metadata{
			Title:     title,
			Artists:   tags["author"],
			Thumbnail: image,
			SourceURL: rawURL,
			URL:       rawURL,
		},
	}

	switch {
	case video != "":
		post.Media = append(post.Media, Media{URL: video, Type: KindVideo, Thumb: image})
	case audio != "":
		post.Media = append(post.Media, Media{URL: audio, Type: KindAudio, Thumb: image})
	default:
		post.Media = append(post.Media, Media{URL: image, Type: KindPhoto, Thumb: image})
	}
	return post
}
