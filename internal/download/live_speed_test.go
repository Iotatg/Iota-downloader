package download

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"
)

// TestLiveDownloadSpeed measures the download speed of the bot against the
// local api.
//
// The test needs the network, it only runs when IOTA_LIVE_TEST is set.
func TestLiveDownloadSpeed(t *testing.T) {
	if os.Getenv("IOTA_LIVE_TEST") == "" {
		t.Skip("set IOTA_LIVE_TEST=1 to run the live speed test")
	}

	key := os.Getenv("IOTA_LIVE_API_KEY")
	url := os.Getenv("IOTA_LIVE_URL")
	if key == "" || url == "" {
		t.Skip("IOTA_LIVE_API_KEY and IOTA_LIVE_URL are needed")
	}

	config := DefaultConfig()
	downloader := NewDownloader(os.TempDir(), config, nil)

	// The old configuration: six workers, a twelve megabyte threshold
	// and the default copy buffer.
	old := DefaultConfig()
	old.MaxWorkers = 6
	old.MultipartThreshold = 12 << 20
	oldDownloader := NewDownloader(os.TempDir(), old, nil)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	for _, run := range []struct {
		name string
		d    *Downloader
		file string
	}{
		{"old", oldDownloader, "live_speed_old.mp4"},
		{"new", downloader, "live_speed_new.mp4"},
	} {
		started := time.Now()
		metrics, err := run.d.Download(ctx, url, run.file, Options{
			Headers: map[string]string{"X-API-Key": key},
		})
		if err != nil {
			t.Fatalf("%s: download failed: %v", run.name, err)
		}
		elapsed := time.Since(started)
		speed := float64(metrics.Size) / elapsed.Seconds()
		t.Logf("%s: size=%d elapsed=%s speed=%.2f MB/s multipart=%v",
			run.name, metrics.Size, elapsed.Round(time.Millisecond),
			speed/(1<<20), metrics.UsedMultipart)
		fmt.Printf("SPEED %s %.2f MB/s in %s (multipart=%v)\n", run.name,
			speed/(1<<20), elapsed.Round(time.Millisecond), metrics.UsedMultipart)
	}
}
