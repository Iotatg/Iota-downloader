package media

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/Iotatg/iota-downloader/internal/httpx"
)

// Limits of a Telegram message thumbnail.
const (
	// ThumbnailMaxEdge is the maximum width and height of a thumbnail.
	ThumbnailMaxEdge = 320
	// ThumbnailMaxBytes is the maximum size of a thumbnail.
	ThumbnailMaxBytes = 200 * 1024
	// thumbnailFetchBytes is the maximum size of a downloaded cover.
	thumbnailFetchBytes = 16 << 20
)

// thumbnailQualities holds the JPEG quality steps used to reach the size
// limit of Telegram.
var thumbnailQualities = []string{"3", "6", "10", "16"}

// FetchThumbnail downloads a cover image into dir and returns its path.
//
// An empty URL or a failed download returns an empty path and an error, the
// callers continue without a cover in that case.
func FetchThumbnail(ctx context.Context, url, dir string) (string, error) {
	if url == "" || dir == "" {
		return "", nil
	}
	if !strings.HasPrefix(url, "http://") && !strings.HasPrefix(url, "https://") {
		return "", nil
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	for key, value := range httpx.BrowserHeaders() {
		request.Header.Set(key, value)
	}

	client := &http.Client{Timeout: 20 * time.Second}
	response, err := client.Do(request)
	if err != nil {
		return "", err
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		return "", fmt.Errorf("cover download failed with status %d", response.StatusCode)
	}

	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	target := filepath.Join(dir, "cover"+coverExtension(response.Header.Get("Content-Type"), url))

	file, err := os.Create(target)
	if err != nil {
		return "", err
	}
	defer file.Close()

	written, err := io.Copy(file, io.LimitReader(response.Body, thumbnailFetchBytes))
	if err != nil {
		return "", err
	}
	if written == 0 {
		return "", fmt.Errorf("cover download is empty")
	}
	return target, nil
}

// PrepareThumbnail converts an image into a thumbnail that Telegram accepts.
//
// The result is a JPEG that fits into 320x320 pixels and stays below 200 kB.
func PrepareThumbnail(ctx context.Context, path string) (string, error) {
	if path == "" {
		return "", nil
	}
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		return path, nil
	}
	if _, err := os.Stat(path); err != nil {
		return "", err
	}

	base := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	target := filepath.Join(filepath.Dir(path), "thumb-"+base+".jpg")

	filter := "scale='min(" + strconv.Itoa(ThumbnailMaxEdge) + ",iw)':'min(" +
		strconv.Itoa(ThumbnailMaxEdge) + ",ih)':force_original_aspect_ratio=decrease"
	force := "scale='min(" + strconv.Itoa(ThumbnailMaxEdge) + ",iw)':-2"

	for _, quality := range thumbnailQualities {
		args := []string{"-y", "-i", path, "-vf", filter, "-frames:v", "1", "-q:v", quality}
		if quality == thumbnailQualities[len(thumbnailQualities)-1] {
			args = []string{"-y", "-i", path, "-vf", force, "-frames:v", "1", "-q:v", quality}
		}
		args = append(args, target)

		command := exec.CommandContext(ctx, "ffmpeg", args...)
		if err := command.Run(); err != nil {
			_ = os.Remove(target)
			return "", err
		}
		produced, err := os.Stat(target)
		if err != nil {
			return "", err
		}
		if produced.Size() <= ThumbnailMaxBytes {
			return target, nil
		}
	}

	if produced, err := os.Stat(target); err == nil && produced.Size() <= ThumbnailMaxBytes {
		return target, nil
	}
	return "", fmt.Errorf("thumbnail is larger than %d bytes", ThumbnailMaxBytes)
}

// Thumbnail downloads a cover and prepares it for Telegram.
func Thumbnail(ctx context.Context, url, dir string) (string, error) {
	downloaded, err := FetchThumbnail(ctx, url, dir)
	if err != nil {
		return "", err
	}
	if downloaded == "" {
		return "", nil
	}
	return PrepareThumbnail(ctx, downloaded)
}

// coverExtension guesses the file extension of a cover image.
func coverExtension(contentType, url string) string {
	switch strings.ToLower(contentType) {
	case "image/png":
		return ".png"
	case "image/webp":
		return ".webp"
	case "image/gif":
		return ".gif"
	}
	lower := strings.ToLower(url)
	switch {
	case strings.Contains(lower, ".png"):
		return ".png"
	case strings.Contains(lower, ".webp"):
		return ".webp"
	case strings.Contains(lower, ".gif"):
		return ".gif"
	default:
		return ".jpg"
	}
}
