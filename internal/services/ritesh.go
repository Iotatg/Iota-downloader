package services

import (
	"context"
	"fmt"
	"time"

	"github.com/Iotatg/iota-downloader/internal/platforms"
)

// riteshFallbackServices lists the platforms whose own resolver may be
// replaced by the Ritesh api.
//
// The api serves the media of YouTube and of the music platforms, it knows
// nothing about the social networks.
var riteshFallbackServices = map[string]bool{
	platforms.ServiceYouTube:     true,
	platforms.ServiceSpotify:     true,
	platforms.ServiceSoundCloud:  true,
	platforms.ServiceDailymotion: true,
}

// riteshAudioPlatforms lists the platforms that always answer with a track.
var riteshAudioPlatforms = map[string]bool{
	platforms.ServiceSpotify:    true,
	platforms.ServiceSoundCloud: true,
	platforms.ServiceAppleMusic: true,
	platforms.ServiceJioSaavn:   true,
	platforms.ServiceDeezer:     true,
	platforms.ServiceGaana:      true,
	platforms.ServiceTidal:      true,
}

// resolveWithRitesh resolves a link with the Ritesh api.
//
// It is the primary resolver of the music platforms that have no public
// download link and the fallback of the platforms whose own resolver is
// blocked.
func (r *Registry) resolveWithRitesh(ctx context.Context, service, rawURL, kind string) (*Resolved, error) {
	if r.Ritesh == nil || !r.Ritesh.Enabled() {
		return nil, fmt.Errorf("the ritesh api is not configured")
	}

	extract, err := r.Ritesh.Extract(ctx, rawURL)
	if err != nil {
		return nil, err
	}

	post := &platforms.Post{
		ID:          extract.ID,
		Service:     service,
		Author:      extract.Author,
		Description: extract.Description,
		Metadata: &platforms.Metadata{
			Title:     extract.Title,
			Artists:   extract.Author,
			Thumbnail: extract.Thumbnail,
			Duration:  time.Duration(extract.Duration) * time.Second,
			SourceURL: extract.URL,
			URL:       extract.URL,
		},
	}

	media := platforms.Media{Thumb: extract.Thumbnail}
	wantAudio := kind == platforms.KindAudio || riteshAudioPlatforms[service]

	switch {
	case wantAudio:
		id := r.Ritesh.AudioID(extract)
		if id == "" {
			return nil, fmt.Errorf("the ritesh api found no full length track")
		}
		media.URL = r.Ritesh.DownloadURL(id, "audio")
		media.Type = platforms.KindAudio
	case r.Ritesh.HasVideo(extract):
		id := r.Ritesh.VideoID(extract)
		if id == "" {
			return nil, fmt.Errorf("the ritesh api found no video")
		}
		media.URL = r.Ritesh.DownloadURL(id, "video")
		media.Type = platforms.KindVideo
	default:
		id := r.Ritesh.AudioID(extract)
		if id == "" {
			return nil, fmt.Errorf("the ritesh api found no media")
		}
		media.URL = r.Ritesh.DownloadURL(id, "audio")
		media.Type = platforms.KindAudio
	}

	if media.URL == "" {
		return nil, fmt.Errorf("the ritesh api returned no media")
	}

	post.Media = append(post.Media, media)
	return &Resolved{
		Service:   service,
		Post:      post,
		AudioOnly: media.Type == platforms.KindAudio,
		Backend:   platforms.BackendRitesh,
	}, nil
}

// canUseRitesh reports whether the Ritesh api may serve the platform.
func (r *Registry) canUseRitesh(service string) bool {
	if r.Ritesh == nil || !r.Ritesh.Enabled() {
		return false
	}
	if riteshAudioPlatforms[service] {
		return true
	}
	return riteshFallbackServices[service]
}

// resolveSoundCloudViaRiteshSearch finds a SoundCloud track with the search
// of the api.
//
// The search of the api looks on youtube, the audio of the video is the
// track. It is the last resort before the Cobalt backend.
func (r *Registry) resolveSoundCloudViaRiteshSearch(ctx context.Context, rawURL string) (*Resolved, error) {
	if r.Ritesh == nil || !r.Ritesh.Enabled() {
		return nil, fmt.Errorf("the ritesh api is not configured")
	}
	if r.SoundCloud == nil {
		return nil, fmt.Errorf("the soundcloud service is not ready")
	}

	metadata, err := r.SoundCloud.FetchMetadata(ctx, rawURL)
	if err != nil {
		// The oEmbed endpoint does not answer for every track, the url of the
		// track still carries its title and its uploader.
		metadata = platforms.SlugMetadata(rawURL)
		if metadata == nil {
			return nil, err
		}
		if r.Log != nil {
			r.Log.Error("soundcloud metadata failed, the url is used: %v", err)
		}
	}

	results, err := r.Ritesh.Search(ctx, metadata.SearchQuery(), 5)
	if err != nil {
		return nil, err
	}
	if len(results) == 0 {
		return nil, fmt.Errorf("the ritesh api found no match for %q", metadata.Title)
	}

	best := results[0]
	bestScore := platforms.MatchScore(metadata.Title, metadata.Artist, best.Title, best.Channel)
	for _, candidate := range results[1:] {
		score := platforms.MatchScore(metadata.Title, metadata.Artist, candidate.Title, candidate.Channel)
		if score > bestScore {
			best = candidate
			bestScore = score
		}
	}

	post := &platforms.Post{
		ID:          best.ID,
		Service:     platforms.ServiceSoundCloud,
		Author:      firstNonEmpty(best.Channel, metadata.Artist),
		Description: firstNonEmpty(best.Title, metadata.Title),
		Metadata: &platforms.Metadata{
			Title:     firstNonEmpty(best.Title, metadata.Title),
			Artists:   firstNonEmpty(best.Channel, metadata.Artist),
			Thumbnail: firstNonEmpty(best.Thumb, metadata.Thumbnail),
			Duration:  time.Duration(best.Duration) * time.Second,
			SourceURL: rawURL,
			URL:       rawURL,
		},
	}
	post.Media = append(post.Media, platforms.Media{
		URL:   r.Ritesh.DownloadURL(best.ID, "audio"),
		Type:  platforms.KindAudio,
		Thumb: post.Metadata.Thumbnail,
	})

	return &Resolved{
		Service:   platforms.ServiceSoundCloud,
		Post:      post,
		AudioOnly: true,
		Backend:   platforms.BackendRitesh,
	}, nil
}
