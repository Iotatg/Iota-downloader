package services

import (
	"context"
	"fmt"
	"time"

	"github.com/Iotatg/iota-downloader/internal/platforms"
)

// resolveResso resolves a Resso track through the iota api.
//
// Resso has no public download link, the api knows the track of the catalog
// and proxies the audio of the matching youtube video.
func (r *Registry) resolveResso(ctx context.Context, rawURL string) (*Resolved, error) {
	trackID, ok := platforms.RessoTrackID(rawURL)
	if !ok {
		return nil, fmt.Errorf("unsupported resso link")
	}
	if r.IotaAPI == nil || !r.IotaAPI.Enabled() {
		return nil, fmt.Errorf("the iota api is not configured")
	}

	mediaID := platforms.RessoMediaID(trackID)
	info, err := r.IotaAPI.Info(ctx, mediaID)
	if err != nil {
		return nil, err
	}

	post := &platforms.Post{
		ID:          trackID,
		Service:     platforms.ServiceResso,
		Author:      info.Artist,
		Description: info.Title,
		Metadata: &platforms.Metadata{
			Title:     info.Title,
			Artists:   info.Artist,
			Duration:  time.Duration(info.Duration) * time.Second,
			Thumbnail: info.Thumbnail,
			SourceURL: rawURL,
			URL:       rawURL,
		},
	}
	post.Media = append(post.Media, platforms.Media{
		URL:   r.IotaAPI.StreamURL(mediaID),
		Type:  platforms.KindAudio,
		Thumb: info.Thumbnail,
	})

	return &Resolved{
		Service:   platforms.ServiceResso,
		Post:      post,
		AudioOnly: true,
		Headers:   r.IotaAPI.Headers(),
	}, nil
}
