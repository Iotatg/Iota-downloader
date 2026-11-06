package services

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Iotatg/iota-downloader/internal/platforms"
	"github.com/Iotatg/iota-downloader/internal/settings"
)

// TestLiveExtraPlatformDownload runs the download path of the bot against a
// public page of an extra platform.
//
// The test needs the network, it only runs when IOTA_LIVE_TEST is set.
func TestLiveExtraPlatformDownload(t *testing.T) {
	if os.Getenv("IOTA_LIVE_TEST") == "" {
		t.Skip("set IOTA_LIVE_TEST=1 to run the live download")
	}

	cookies := os.Getenv("IOTA_LIVE_COOKIES")
	ytdlp := platforms.NewYTDLP(platforms.YTDLPOptions{
		CookiesFile:    cookies,
		InfoTimeout:    60 * time.Second,
		FFMPEGLocation: os.Getenv("FFMPEG_LOCATION"),
	}, nil)

	registry := &Registry{
		YouTube: platforms.NewYouTubeService(ytdlp, os.TempDir(), nil),
	}

	cases := []struct {
		service string
		url     string
	}{
		{platforms.ServiceWistia, "https://home.wistia.com/medias/e4a27b971d"},
		{platforms.ServiceStreamable, "https://streamable.com/ifjh"},
		{platforms.ServiceCoub, "https://coub.com/view/1h7z1"},
	}

	for _, item := range cases {
		ctx, cancel := context.WithTimeout(context.Background(), 150*time.Second)
		resolved, err := registry.resolveWithYTDLPBackend(ctx, item.service, item.url)
		if err != nil {
			cancel()
			t.Errorf("%s: resolve failed: %v", item.service, err)
			continue
		}
		if resolved.Backend != platforms.BackendYTDLP {
			t.Errorf("%s: backend = %q, want %q", item.service, resolved.Backend, platforms.BackendYTDLP)
		}
		if len(resolved.Post.Media) == 0 {
			t.Errorf("%s: no media", item.service)
			cancel()
			continue
		}

		// The bot downloads with the generic selector of an extra platform.
		selector := settings.GenericVideoFormatSelector(settings.Balanced)
		name := filepath.Join(os.TempDir(), "live_"+item.service)
		page := resolved.Post.Media[0].URL
		path, err := registry.YouTube.DownloadVideo(ctx, page, name, selector, 0)
		cancel()
		if err != nil {
			t.Errorf("%s: download failed: %v", item.service, err)
			continue
		}
		info, err := os.Stat(path)
		if err != nil || info.Size() < 100_000 {
			t.Errorf("%s: file too small (%v)", item.service, err)
			continue
		}
		t.Logf("%s ok: %s (%d bytes)", item.service, path, info.Size())
	}
}
