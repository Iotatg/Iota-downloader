// Package services wires the platform resolvers together and exposes a single
// entry point used by the handlers to resolve a link into a media post.
package services

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/Iotatg/iota-downloader/internal/platforms"
)

// backendError marks a failure that the Ritesh api cannot repair.
//
// The fallback of the bot cannot help when the own backend of the bot is
// offline, so the answer of the user must name the missing backend instead
// of a generic failure.
type backendError struct {
	err error
}

func (e *backendError) Error() string { return e.err.Error() }

func (e *backendError) Unwrap() error { return e.err }

// isBackendError reports whether the error names a missing backend.
func isBackendError(err error) bool {
	var backend *backendError
	return errors.As(err, &backend)
}

// Logger is the minimal logging surface used by the registry.
type Logger interface {
	Debug(format string, args ...any)
	Info(format string, args ...any)
	Error(format string, args ...any)
}

// Registry holds every platform resolver of the bot.
type Registry struct {
	YouTube    *platforms.YouTubeService
	TikTok     *platforms.TikTokService
	Twitter    *platforms.TwitterService
	Threads    *platforms.ThreadsService
	Instagram  *platforms.InstagramService
	Pinterest  *platforms.PinterestService
	SoundCloud *platforms.SoundCloudService
	Spotify    *platforms.SpotifyService
	Mastodon   *platforms.MastodonService
	IotaAPI    *platforms.IotaAPIService
	Ritesh     *platforms.RiteshService
	Deezer     *platforms.DeezerService

	// YTDLP reads the playlists of the platforms, it is the generic reader
	// of the collections.
	YTDLP *platforms.YTDLP

	// Cache keeps the recent resolutions, so a link that is sent twice does
	// not pay for the resolver twice.
	Cache *ResolutionCache

	// Collections keeps the expanded collections, the menu of a collection
	// holds the entries of the collection.
	Collections *CollectionCache

	Log Logger
}

// Resolved is the outcome of a link resolution.
type Resolved struct {
	// Service is the platform identifier of the resolved link.
	Service string
	// Post holds the resolved media post.
	Post *platforms.Post
	// Track is set when the link points to an audio only source.
	Track *platforms.TrackInfo
	// AudioURL is the direct audio link of an audio only source.
	AudioURL string
	// AudioOnly reports whether the resolved item is an audio track.
	AudioOnly bool
	// Headers are the request headers needed to fetch the media.
	Headers map[string]string
	// Cached reports whether the resolution came from the cache.
	Cached bool
	// Backend names the resolver that delivered the media. It is empty when
	// the own resolver of the platform was used.
	Backend string
}

// Resolve turns a raw link into a media post of the matching platform.
//
// A cached resolution is returned as it is, the resolvers of the platforms
// are the slow part of a download.
func (r *Registry) Resolve(ctx context.Context, service, rawURL string) (*Resolved, error) {
	return r.ResolveKind(ctx, service, rawURL, platforms.KindVideo)
}

// ResolveKind turns a raw link into a media post of the matching platform and
// asks for a certain kind of media.
//
// The kind is "video" or "audio", the music platforms always answer with an
// audio track.
func (r *Registry) ResolveKind(ctx context.Context, service, rawURL, kind string) (*Resolved, error) {
	key := service + "|" + kind + "|" + rawURL
	if cached := r.Cache.Get(key); cached != nil {
		cached.Cached = true
		return cached, nil
	}

	resolved, err := r.resolve(ctx, service, rawURL)
	if err != nil {
		// The Ritesh api is the fallback of the bot, it answers when the
		// own resolver of the platform is blocked. It cannot help when the
		// backend of the bot is offline.
		if r.fallbackAllowed(err, service) {
			if fallback, fallbackErr := r.resolveWithRitesh(ctx, service, rawURL, kind); fallbackErr == nil {
				r.Cache.Put(key, fallback)
				return fallback, nil
			}
		}
		return nil, err
	}
	r.Cache.Put(key, resolved)
	return resolved, nil
}

// fallbackAllowed reports whether the Ritesh api may answer the failure of
// a resolver.
//
// The api cannot repair a missing backend, so the answer of the user must
// name the backend instead of trying another route.
func (r *Registry) fallbackAllowed(err error, service string) bool {
	if isBackendError(err) {
		return false
	}
	return r.canUseRitesh(service)
}

// resolve asks the resolver of the platform for the media of a link.
func (r *Registry) resolve(ctx context.Context, service, rawURL string) (*Resolved, error) {
	switch service {
	case platforms.ServiceYouTube:
		if r.YouTube == nil {
			return nil, fmt.Errorf("the youtube service is not ready")
		}
		post, err := r.YouTube.FetchVideo(ctx, rawURL)
		if err != nil {
			return nil, err
		}
		return &Resolved{Service: service, Post: post}, nil

	case platforms.ServiceTikTok:
		return r.resolveTikTok(ctx, rawURL)

	case platforms.ServiceTwitter:
		return r.resolveTwitter(ctx, rawURL)

	case platforms.ServiceThreads:
		if r.Threads == nil {
			return nil, fmt.Errorf("the threads service is not ready")
		}
		post, err := r.Threads.FetchPost(ctx, rawURL)
		if err != nil {
			return nil, err
		}
		return &Resolved{Service: service, Post: post}, nil

	case platforms.ServiceInstagram:
		if r.Instagram == nil {
			return nil, fmt.Errorf("the instagram service is not ready")
		}
		post, err := r.Instagram.FetchPost(ctx, rawURL)
		if err != nil {
			return nil, err
		}
		return &Resolved{Service: service, Post: post}, nil

	case platforms.ServicePinterest:
		if r.Pinterest == nil {
			return nil, fmt.Errorf("the pinterest service is not ready")
		}
		post, err := r.Pinterest.FetchPost(ctx, rawURL)
		if err != nil {
			return nil, err
		}
		return &Resolved{Service: service, Post: post}, nil

	case platforms.ServiceSoundCloud:
		return r.resolveSoundCloud(ctx, rawURL)

	case platforms.ServiceSpotify:
		return r.resolveSpotify(ctx, rawURL)

	case platforms.ServiceAppleMusic,
		platforms.ServiceJioSaavn,
		platforms.ServiceDeezer,
		platforms.ServiceGaana,
		platforms.ServiceTidal:
		return r.resolveWithRitesh(ctx, service, rawURL, platforms.KindAudio)

	case platforms.ServiceDailymotion:
		return r.resolveWithYTDLP(ctx, service, rawURL)

	case platforms.ServiceBluesky,
		platforms.ServiceFacebook,
		platforms.ServiceReddit,
		platforms.ServiceTwitch,
		platforms.ServiceKick,
		platforms.ServiceBilibili,
		platforms.ServiceTumblr,
		platforms.ServiceLinkedIn,
		platforms.ServiceIMDb,
		platforms.ServiceMXPlayer,
		platforms.ServiceSnapchat:
		return r.resolveWithYTDLPBackend(ctx, service, rawURL)

	case platforms.ServiceMastodon:
		if r.Mastodon == nil {
			return nil, fmt.Errorf("the mastodon service is not ready")
		}
		post, err := r.Mastodon.FetchPost(ctx, rawURL)
		if err != nil {
			return nil, err
		}
		return &Resolved{Service: service, Post: post}, nil

	case platforms.ServiceResso:
		return r.resolveResso(ctx, rawURL)

	default:
		return nil, fmt.Errorf("unsupported service: %s", service)
	}
}

// resolveTikTok converts the TikTok payload into a post.
func (r *Registry) resolveTikTok(ctx context.Context, rawURL string) (*Resolved, error) {
	if r.TikTok == nil {
		return nil, fmt.Errorf("the tiktok service is not ready")
	}
	expanded := r.TikTok.ExpandShortURL(ctx, rawURL)
	payload, err := r.TikTok.FetchData(ctx, expanded)
	if err != nil {
		return nil, err
	}

	post := &platforms.Post{
		ID:          payload.ID,
		Service:     platforms.ServiceTikTok,
		Author:      payload.Author,
		Description: payload.Title,
		Metadata: &platforms.Metadata{
			Title:     payload.Title,
			Artists:   payload.Author,
			Thumbnail: payload.Cover,
			SourceURL: payload.WebpageURL,
			URL:       payload.WebpageURL,
		},
		Info: &platforms.VideoInfo{
			ID:              payload.ID,
			Description:     payload.Title,
			Author:          payload.Author,
			Cover:           payload.Cover,
			Views:           payload.PlayCount,
			Likes:           payload.DiggCount,
			Comments:        payload.CommentCount,
			Shares:          payload.ShareCount,
			DurationSeconds: payload.Duration,
			SizeHint:        payload.Size,
		},
	}

	if len(payload.Images) > 0 {
		for index, image := range payload.Images {
			post.Media = append(post.Media, platforms.Media{
				URL:   image,
				Type:  platforms.KindPhoto,
				Thumb: payload.Cover,
			})
			_ = index
		}
		post.Album = len(post.Media) > 1
		return &Resolved{Service: platforms.ServiceTikTok, Post: post}, nil
	}

	play := payload.Play
	if play == "" {
		play = payload.HDPlay
	}
	if play == "" {
		play = payload.WatermarkPlay
	}
	if play == "" {
		return nil, fmt.Errorf("no playable media found")
	}
	post.Media = append(post.Media, platforms.Media{URL: play, Type: platforms.KindVideo, Thumb: payload.Cover})
	return &Resolved{Service: platforms.ServiceTikTok, Post: post}, nil
}

// resolveTwitter converts a tweet into a post.
func (r *Registry) resolveTwitter(ctx context.Context, rawURL string) (*Resolved, error) {
	if r.Twitter == nil {
		return nil, fmt.Errorf("the twitter service is not ready")
	}
	tweet, err := r.Twitter.FetchPost(ctx, rawURL)
	if err != nil {
		return nil, err
	}

	post := &platforms.Post{
		ID:          tweet.ID,
		Service:     platforms.ServiceTwitter,
		Author:      tweet.AuthorName,
		Description: tweet.Text,
		Metadata: &platforms.Metadata{
			Title:     tweet.Text,
			Artists:   tweet.AuthorName,
			Thumbnail: tweetThumbnail(tweet),
			SourceURL: tweet.URL,
			URL:       tweet.URL,
		},
	}
	for _, item := range tweet.Media {
		post.Media = append(post.Media, item)
	}
	if len(post.Media) > 1 {
		post.Album = true
	}
	return &Resolved{Service: platforms.ServiceTwitter, Post: post}, nil
}

// tweetThumbnail returns the cover of a tweet.
func tweetThumbnail(tweet *platforms.TwitterPost) string {
	for _, item := range tweet.Media {
		if item.Thumb != "" {
			return item.Thumb
		}
	}
	return ""
}

// resolveSpotify converts a Spotify track into an audio only result.
func (r *Registry) resolveSpotify(ctx context.Context, rawURL string) (*Resolved, error) {
	kind, spotifyID, ok := platforms.ParseSpotifyURL(rawURL)
	if !ok {
		return nil, fmt.Errorf("unsupported spotify link")
	}
	if r.Spotify == nil {
		return nil, fmt.Errorf("the spotify service is not ready")
	}
	if kind != "track" {
		return nil, fmt.Errorf("send a spotify track link, not an album or playlist link")
	}

	track, err := r.Spotify.FetchTrack(ctx, rawURL)
	if err != nil {
		return nil, err
	}

	post := &platforms.Post{
		ID:          track.ID,
		Service:     platforms.ServiceSpotify,
		Author:      track.Artists,
		Description: track.Title,
		Metadata: &platforms.Metadata{
			Title:       track.Title,
			Artists:     track.Artists,
			Album:       track.Album,
			AlbumArtist: track.AlbumArtist,
			Date:        track.Date,
			Thumbnail:   track.Thumbnail,
			SourceURL:   track.SourceURL,
			URL:         track.URL,
		},
	}

	// The api resolves the track on the spotify side and proxies the audio,
	// it is the only source that can deliver a playable track.
	if r.IotaAPI.Enabled() {
		mediaID := "sp:" + spotifyID
		info, err := r.IotaAPI.Info(ctx, mediaID)
		if err != nil {
			if r.Log != nil {
				r.Log.Error("iota api spotify lookup failed: %v", err)
			}
			// The api is the only source of a playable track, the Ritesh
			// api cannot download the track without its own key.
			return nil, &backendError{fmt.Errorf("the music backend did not answer: %w", err)}
		}
		applyIotaMetadata(post, info)
		if post.Metadata.Artists == "" || post.Metadata.Artists == "Unknown artist" {
			post.Metadata.Artists = info.Artist
			post.Author = info.Artist
		}
		if post.Metadata.Title == "" || post.Metadata.Title == "Unknown title" {
			post.Metadata.Title = info.Title
			post.Description = info.Title
		}
		post.Media = append(post.Media, platforms.Media{
			URL:   r.IotaAPI.StreamURL(mediaID),
			Type:  platforms.KindAudio,
			Thumb: post.Metadata.Thumbnail,
		})
		return &Resolved{
			Service:   platforms.ServiceSpotify,
			Post:      post,
			Track:     track,
			AudioOnly: true,
			Headers:   r.IotaAPI.Headers(),
		}, nil
	}

	if track.AudioURL != "" {
		post.Media = append(post.Media, platforms.Media{URL: track.AudioURL, Type: platforms.KindAudio, Thumb: track.Thumbnail})
		return &Resolved{Service: platforms.ServiceSpotify, Post: post, Track: track, AudioOnly: true}, nil
	}
	return nil, &backendError{fmt.Errorf("the music backend is not configured: set iota_api_url and iota_api_key on the server")}
}

// resolveSoundCloud resolves a SoundCloud track. The api is preferred, it
// carries its own resolver and stays up when the public Cobalt instances do
// not answer.
func (r *Registry) resolveSoundCloud(ctx context.Context, rawURL string) (*Resolved, error) {
	if r.IotaAPI.Enabled() {
		resolved, err := r.resolveSoundCloudViaAPI(ctx, rawURL)
		if err == nil {
			return resolved, nil
		}
		if r.Log != nil {
			r.Log.Error("iota api soundcloud lookup failed: %v", err)
		}
	}

	// The Ritesh api knows the track itself.
	if r.canUseRitesh(platforms.ServiceSoundCloud) {
		resolved, err := r.resolveWithRitesh(ctx, platforms.ServiceSoundCloud, rawURL, platforms.KindAudio)
		if err == nil {
			return resolved, nil
		}
		if r.Log != nil {
			r.Log.Error("ritesh soundcloud lookup failed: %v", err)
		}

		// The search of the api finds the track on youtube.
		resolved, err = r.resolveSoundCloudViaRiteshSearch(ctx, rawURL)
		if err == nil {
			return resolved, nil
		}
		if r.Log != nil {
			r.Log.Error("ritesh soundcloud search failed: %v", err)
		}
	}

	post, err := r.SoundCloud.FetchTrack(ctx, rawURL)
	if err != nil {
		return nil, err
	}
	return &Resolved{Service: platforms.ServiceSoundCloud, Post: post, AudioOnly: true}, nil
}

// resolveSoundCloudViaAPI resolves a SoundCloud track through the api.
func (r *Registry) resolveSoundCloudViaAPI(ctx context.Context, rawURL string) (*Resolved, error) {
	metadata, err := r.SoundCloud.FetchMetadata(ctx, rawURL)
	if err != nil {
		return nil, err
	}

	candidates, err := r.IotaAPI.Search(ctx, metadata.SearchQuery(), 5)
	if err != nil {
		return nil, err
	}
	if len(candidates) == 0 {
		return nil, fmt.Errorf("the iota api found no match for %q", metadata.Title)
	}

	best := candidates[0]
	bestScore := platforms.MatchScore(metadata.Title, metadata.Artist, best.Title, best.Artist)
	for _, candidate := range candidates[1:] {
		score := platforms.MatchScore(metadata.Title, metadata.Artist, candidate.Title, candidate.Artist)
		if score > bestScore {
			best = candidate
			bestScore = score
		}
	}

	post := &platforms.Post{
		ID:          best.MediaID,
		Service:     platforms.ServiceSoundCloud,
		Author:      firstNonEmpty(best.Artist, metadata.Artist),
		Description: firstNonEmpty(best.Title, metadata.Title),
		Metadata: &platforms.Metadata{
			Title:     firstNonEmpty(best.Title, metadata.Title),
			Artists:   firstNonEmpty(best.Artist, metadata.Artist),
			Thumbnail: firstNonEmpty(best.Thumbnail, metadata.Thumbnail),
			SourceURL: rawURL,
			URL:       rawURL,
		},
	}
	post.Media = append(post.Media, platforms.Media{
		URL:   r.IotaAPI.StreamURL(best.MediaID),
		Type:  platforms.KindAudio,
		Thumb: post.Metadata.Thumbnail,
	})

	return &Resolved{
		Service:   platforms.ServiceSoundCloud,
		Post:      post,
		AudioOnly: true,
		Headers:   r.IotaAPI.Headers(),
	}, nil
}

// applyIotaMetadata copies the metadata of the api into a post, the values
// that are already known stay untouched.
func applyIotaMetadata(post *platforms.Post, info *platforms.IotaTrack) {
	if post == nil || info == nil {
		return
	}
	if post.Metadata == nil {
		post.Metadata = &platforms.Metadata{}
	}
	if post.Metadata.Title == "" && info.Title != "" {
		post.Metadata.Title = info.Title
	}
	if post.Description == "" {
		post.Description = info.Title
	}
	if post.Metadata.Artists == "" && info.Artist != "" {
		post.Metadata.Artists = info.Artist
	}
	if post.Author == "" {
		post.Author = info.Artist
	}
	if post.Metadata.Thumbnail == "" && info.Thumbnail != "" {
		post.Metadata.Thumbnail = info.Thumbnail
	}
}

// firstNonEmpty returns the first value that is not empty.
func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

// AudioCandidates returns the search terms used to find an audio mirror of a
// Spotify track.
func AudioCandidates(track *platforms.TrackInfo) []string {
	if track == nil {
		return nil
	}
	candidates := []string{}
	if track.Artists != "" && track.Title != "" {
		candidates = append(candidates, track.Artists+" "+track.Title)
	}
	if track.Artists != "" {
		candidates = append(candidates, track.Artists)
	}
	return candidates
}

// NormalizeTitle cleans a title for the file name of a download.
func NormalizeTitle(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "download"
	}
	return platforms.SanitizeFilename(value, 120)
}
