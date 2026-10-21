// Package platforms contains the per-platform media resolvers of the Iota
// Coder bot together with the shared link detection logic.
package platforms

import (
	"net/url"
	"regexp"
	"sort"
	"strings"
)

// Service identifiers of the supported platforms.
const (
	ServiceTikTok     = "tiktok"
	ServiceInstagram  = "instagram"
	ServiceThreads    = "threads"
	ServiceSoundCloud = "soundcloud"
	ServiceSpotify    = "spotify"
	ServicePinterest  = "pinterest"
	ServiceYouTube    = "youtube"
	ServiceTwitter    = "twitter"

	// The platforms below are served by the Ritesh api, they have no public
	// download link of their own.
	ServiceAppleMusic  = "apple_music"
	ServiceJioSaavn    = "jiosaavn"
	ServiceDeezer      = "deezer"
	ServiceGaana       = "gaana"
	ServiceTidal       = "tidal"
	ServiceDailymotion = "dailymotion"

	// The platforms below are resolved with yt-dlp, they serve their media
	// as a protected stream.
	ServiceBluesky  = "bluesky"
	ServiceFacebook = "facebook"
	ServiceReddit   = "reddit"
	ServiceTwitch   = "twitch"
	ServiceKick     = "kick"
	ServiceBilibili = "bilibili"
	ServiceTumblr   = "tumblr"
	ServiceLinkedIn = "linkedin"
	ServiceIMDb     = "imdb"
	ServiceMXPlayer = "mxplayer"
	ServiceSnapchat = "snapchat"

	// The platforms below have a resolver of their own.
	ServiceMastodon = "mastodon"
	ServiceResso    = "resso"
)

// YTDLPServices lists the platforms that yt-dlp knows.
var YTDLPServices = []string{
	ServiceBluesky,
	ServiceFacebook,
	ServiceReddit,
	ServiceTwitch,
	ServiceKick,
	ServiceBilibili,
	ServiceTumblr,
	ServiceLinkedIn,
	ServiceIMDb,
	ServiceMXPlayer,
	ServiceSnapchat,
}

// RiteshServices lists the platforms that the Ritesh api serves.
var RiteshServices = []string{
	ServiceAppleMusic,
	ServiceJioSaavn,
	ServiceDeezer,
	ServiceGaana,
	ServiceTidal,
	ServiceDailymotion,
}

// Services lists every supported platform.
var Services = []string{
	ServiceTikTok,
	ServiceInstagram,
	ServiceThreads,
	ServiceSoundCloud,
	ServiceSpotify,
	ServicePinterest,
	ServiceYouTube,
	ServiceTwitter,
	ServiceAppleMusic,
	ServiceJioSaavn,
	ServiceDeezer,
	ServiceGaana,
	ServiceTidal,
	ServiceDailymotion,
	ServiceBluesky,
	ServiceFacebook,
	ServiceReddit,
	ServiceTwitch,
	ServiceKick,
	ServiceBilibili,
	ServiceTumblr,
	ServiceLinkedIn,
	ServiceIMDb,
	ServiceMXPlayer,
	ServiceSnapchat,
	ServiceMastodon,
	ServiceResso,
}

// ServiceIcon returns the symbol used for a platform.
//
// The bot does not use emoji, every platform wears one of the symbols of the
// Iota Coder look.
func ServiceIcon(service string) string {
	switch service {
	case ServiceTikTok:
		return "☝"
	case ServiceInstagram:
		return "☚"
	case ServiceYouTube:
		return "☞"
	case "youtube_audio", "youtube_music":
		return "★"
	case ServiceTwitter:
		return "☟"
	case ServicePinterest:
		return "◆"
	case ServiceThreads:
		return "☛"
	case ServiceSpotify:
		return "◉"
	case ServiceSoundCloud:
		return "◇"
	case ServiceAppleMusic, ServiceTidal:
		return "◈"
	case ServiceJioSaavn, ServiceGaana:
		return "◐"
	case ServiceDeezer:
		return "◑"
	case ServiceDailymotion:
		return "◓"
	case ServiceBluesky:
		return "⬡"
	case ServiceFacebook:
		return "⬢"
	case ServiceReddit:
		return "⬣"
	case ServiceTwitch:
		return "⬤"
	case ServiceKick:
		return "⬥"
	case ServiceBilibili:
		return "⬦"
	case ServiceTumblr:
		return "⬧"
	case ServiceLinkedIn:
		return "⬨"
	case ServiceIMDb:
		return "⯃"
	case ServiceMXPlayer:
		return "⬔"
	case ServiceSnapchat:
		return "⬕"
	case ServiceMastodon:
		return "⬖"
	case ServiceResso:
		return "⬗"
	default:
		return "▫"
	}
}

// ServiceLabel returns the display name of a platform.
func ServiceLabel(service string) string {
	switch service {
	case ServiceTikTok:
		return "TikTok"
	case ServiceInstagram:
		return "Instagram"
	case ServiceThreads:
		return "Threads"
	case ServiceSoundCloud:
		return "SoundCloud"
	case ServiceSpotify:
		return "Spotify"
	case ServicePinterest:
		return "Pinterest"
	case ServiceYouTube:
		return "YouTube"
	case ServiceTwitter:
		return "Twitter"
	case ServiceAppleMusic:
		return "Apple Music"
	case ServiceJioSaavn:
		return "JioSaavn"
	case ServiceDeezer:
		return "Deezer"
	case ServiceGaana:
		return "Gaana"
	case ServiceTidal:
		return "Tidal"
	case ServiceDailymotion:
		return "Dailymotion"
	case ServiceBluesky:
		return "Bluesky"
	case ServiceFacebook:
		return "Facebook"
	case ServiceReddit:
		return "Reddit"
	case ServiceTwitch:
		return "Twitch"
	case ServiceKick:
		return "Kick"
	case ServiceBilibili:
		return "Bilibili"
	case ServiceTumblr:
		return "Tumblr"
	case ServiceLinkedIn:
		return "LinkedIn"
	case ServiceIMDb:
		return "IMDb"
	case ServiceMXPlayer:
		return "MX Player"
	case ServiceSnapchat:
		return "Snapchat"
	case ServiceMastodon:
		return "Mastodon"
	case ServiceResso:
		return "Resso"
	default:
		return service
	}
}

type servicePattern struct {
	service string
	regex   *regexp.Regexp
}

// servicePatterns lists the detection patterns in matching order.
var servicePatterns = []servicePattern{
	{ServiceTikTok, regexp.MustCompile(`https?://(?:www\.|vm\.|vt\.|vn\.)?tiktok\.com/\S+`)},
	{ServiceInstagram, regexp.MustCompile(`https?://(?:www\.)?instagram\.com/\S+`)},
	{ServiceThreads, regexp.MustCompile(`https?://(?:www\.)?threads\.(?:com|net)/@[A-Za-z0-9._-]+/post/[A-Za-z0-9_-]+`)},
	{ServiceSoundCloud, regexp.MustCompile(`https?://(?:www\.|m\.)?soundcloud\.com/\S+|https?://on\.soundcloud\.com/\S+|https?://soundcloud\.app\.goo\.gl/\S+`)},
	{ServiceSpotify, regexp.MustCompile(`https?://(?:(?:open|www)\.)?spotify\.com/(?:intl-[a-z]{2}/)?track/[A-Za-z0-9]+(?:\?\S*)?`)},
	{ServicePinterest, regexp.MustCompile(`https?://(?:[\w-]+\.)?pinterest\.[\w.]+/\S+|https?://pin\.it/\S+`)},
	{ServiceYouTube, regexp.MustCompile(`https?://(?:www\.|music\.)?(?:youtube|youtu|youtube-nocookie)\.(?:com|be)/\S+`)},
	{ServiceTwitter, regexp.MustCompile(`https?://(?:www\.)?(?:twitter|x)\.com/\S+|https?://t\.co/\S+`)},
	{ServiceAppleMusic, regexp.MustCompile(`https?://(?:music|itunes)\.apple\.com/\S+`)},
	{ServiceJioSaavn, regexp.MustCompile(`https?://(?:www\.)?jiosaavn\.com/\S+`)},
	{ServiceDeezer, regexp.MustCompile(`https?://(?:www\.)?deezer\.com/\S+`)},
	{ServiceGaana, regexp.MustCompile(`https?://(?:www\.)?gaana\.com/\S+`)},
	{ServiceTidal, regexp.MustCompile(`https?://(?:www\.)?tidal\.com/\S+`)},
	{ServiceDailymotion, regexp.MustCompile(`https?://(?:www\.)?dailymotion\.com/\S+`)},
	{ServiceBluesky, regexp.MustCompile(`https?://(?:www\.)?bsky\.app/profile/[\w.:%-]+/post/\w+`)},
	{ServiceFacebook, regexp.MustCompile(`https?://(?:[\w-]+\.)?facebook\.com/\S+|https?://(?:www\.)?fb\.watch/\S+`)},
	{ServiceReddit, regexp.MustCompile(`https?://(?:[\w-]+\.)?reddit(?:media)?\.com/\S+|https?://(?:www\.)?redd\.it/\S+|https?://v\.redd\.it/\S+`)},
	{ServiceTwitch, regexp.MustCompile(`https?://(?:[\w-]+\.)?twitch\.tv/\S+`)},
	{ServiceKick, regexp.MustCompile(`https?://(?:www\.)?kick\.com/\S+`)},
	{ServiceBilibili, regexp.MustCompile(`https?://(?:[\w-]+\.)?bilibili\.com/\S+|https?://b23\.tv/\S+`)},
	{ServiceTumblr, regexp.MustCompile(`https?://(?:[\w-]+\.)?tumblr\.com/\S+`)},
	{ServiceLinkedIn, regexp.MustCompile(`https?://(?:www\.)?linkedin\.com/(?:posts|feed)/\S+`)},
	{ServiceIMDb, regexp.MustCompile(`https?://(?:www\.|m\.)?imdb\.com/(?:video|title)/\S+`)},
	{ServiceMXPlayer, regexp.MustCompile(`https?://(?:www\.)?mxplayer\.in/\S+`)},
	{ServiceSnapchat, regexp.MustCompile(`https?://(?:www\.)?snapchat\.com/spotlight/\w+`)},
	{ServiceResso, regexp.MustCompile(`https?://(?:www\.|m\.)?resso\.com/\S+`)},

	// The mastodon patterns are the most generic ones, they stay at the end
	// of the list so that a known platform wins over them.
	{ServiceMastodon, regexp.MustCompile(`https?://(?:[\w-]+\.)*(?:mastodon|mstdn|fosstodon|hachyderm)[\w-]*\.[a-z]{2,}/@[\w.-]+/\d+`)},
	{ServiceMastodon, regexp.MustCompile(`https?://(?:mas\.to|techhub\.social|influx\.social|northboot\.social|chaos\.social|ruby\.social|nerd\.horse|det\.social|social\.linux\.pizza|fedi\.privacydev\.net)/@[\w.-]+/\d+`)},
}

const trailingPunctuation = ".,;:!?)]}>\"'"

// DetectService returns the platform of the first supported link in text.
func DetectService(text string) (string, bool) {
	service, _, ok := ExtractLink(text)
	return service, ok
}

// ExtractLink returns the platform and the canonical URL of the first
// supported link inside text.
func ExtractLink(text string) (string, string, bool) {
	links := ExtractLinks(text)
	if len(links) == 0 {
		return "", "", false
	}
	return links[0].Service, links[0].URL, true
}

// Link is a supported link found inside a message.
type Link struct {
	Service string
	URL     string
	Index   int
}

// ExtractLinks returns every supported link of a message ordered by position.
func ExtractLinks(text string) []Link {
	if strings.TrimSpace(text) == "" {
		return nil
	}

	type match struct {
		service string
		url     string
		start   int
	}
	var matches []match

	for _, pattern := range servicePatterns {
		for _, found := range pattern.regex.FindAllStringIndex(text, -1) {
			matches = append(matches, match{
				service: pattern.service,
				url:     strings.TrimRight(text[found[0]:found[1]], trailingPunctuation),
				start:   found[0],
			})
		}
	}

	sort.SliceStable(matches, func(i, j int) bool { return matches[i].start < matches[j].start })

	seen := map[string]bool{}
	var links []Link
	for _, item := range matches {
		canonical := CanonicalizeURL(item.service, item.url)
		if canonical == "" || seen[canonical] {
			continue
		}
		seen[canonical] = true
		links = append(links, Link{Service: item.service, URL: canonical, Index: item.start})
	}
	return links
}

// CanonicalizeURL normalises a supported URL. Query parameters are dropped
// for every platform except YouTube, which keeps the video relevant ones.
func CanonicalizeURL(service, rawURL string) string {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return ""
	}
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return ""
	}
	if parsed.Host == "" {
		return ""
	}

	scheme := strings.ToLower(parsed.Scheme)
	host := strings.ToLower(parsed.Host)
	path := parsed.EscapedPath()
	if path == "" {
		path = "/"
	}

	if service == ServiceYouTube {
		query := parsed.Query()
		kept := url.Values{}
		for _, key := range []string{"v", "list", "t", "start"} {
			if value := query.Get(key); value != "" {
				kept.Set(key, value)
			}
		}
		if encoded := kept.Encode(); encoded != "" {
			return scheme + "://" + host + path + "?" + encoded
		}
		return scheme + "://" + host + path
	}

	return scheme + "://" + host + path
}

// StripURLQuery removes the query string and the fragment of a URL.
func StripURLQuery(rawURL string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return rawURL
	}
	parsed.RawQuery = ""
	parsed.Fragment = ""
	return parsed.String()
}

// IsYouTubeMusic reports whether a YouTube URL points at YouTube Music.
func IsYouTubeMusic(rawURL string) bool {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return false
	}
	return strings.HasPrefix(strings.ToLower(parsed.Hostname()), "music.")
}
