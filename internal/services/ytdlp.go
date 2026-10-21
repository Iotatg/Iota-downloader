package services

import (
	"context"
	"fmt"

	"github.com/Iotatg/iota-downloader/internal/platforms"
)

// resolveWithYTDLP resolves a video platform with yt-dlp.
//
// yt-dlp knows the video platforms of the web, the Ritesh api is the
// fallback for it.
func (r *Registry) resolveWithYTDLP(ctx context.Context, service, rawURL string) (*Resolved, error) {
	if r.YouTube == nil {
		return nil, fmt.Errorf("the yt-dlp service is not ready")
	}

	post, err := r.YouTube.FetchVideo(ctx, rawURL)
	if err != nil {
		return nil, err
	}
	post.Service = service
	return &Resolved{Service: service, Post: post}, nil
}

// resolveWithYTDLPBackend resolves a video platform with yt-dlp and lets
// yt-dlp do the download as well.
//
// The platforms that serve their media as a protected stream need yt-dlp for
// the download: the direct stream of the resolver carries a session of the
// platform and cannot be fetched as a plain file.
func (r *Registry) resolveWithYTDLPBackend(ctx context.Context, service, rawURL string) (*Resolved, error) {
	if r.YouTube == nil {
		return nil, fmt.Errorf("the yt-dlp service is not ready")
	}

	post, info, err := r.YouTube.FetchVideoWithInfo(ctx, rawURL)
	if err != nil {
		return nil, err
	}
	post.Service = service

	// A post without a video stream is a photo post, the file of the platform
	// is fetched directly.
	if kind, direct := platforms.PhotoMediaFromInfo(info); kind != "" {
		post.Media = []platforms.Media{{URL: direct, Type: kind, Thumb: direct}}
		return &Resolved{Service: service, Post: post}, nil
	}

	// The download runs with yt-dlp again, the media of the post keeps the
	// page of the platform so that the extractor finds the stream.
	page := rawURL
	if post.Metadata != nil && post.Metadata.SourceURL != "" {
		page = post.Metadata.SourceURL
	}
	for index := range post.Media {
		post.Media[index].URL = page
	}

	return &Resolved{Service: service, Post: post, Backend: platforms.BackendYTDLP}, nil
}
