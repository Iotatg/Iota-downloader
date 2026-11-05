package services

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Iotatg/iota-downloader/internal/platforms"
)

// iotaTestServer mimics the iota-api: /info/<media id> carries the
// metadata, /download/<media id> serves the bytes.
func iotaTestServer(t *testing.T, infoStatus int) (*httptest.Server, *int) {
	t.Helper()
	infoCalls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasPrefix(r.URL.Path, "/info/"):
			infoCalls++
			if infoStatus != http.StatusOK {
				w.WriteHeader(infoStatus)
				_, _ = w.Write([]byte(`{"detail":"extraction failed"}`))
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"media_id":         "yt:abc12345678",
				"title":            "Track Title",
				"artist":           "Track Artist",
				"duration_seconds": 120,
				"provider":         "yt",
			})
		case strings.HasPrefix(r.URL.Path, "/download/"):
			w.WriteHeader(http.StatusOK)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	return server, &infoCalls
}

func TestResolveYouTubeUsesTheIotaProvider(t *testing.T) {
	server, infoCalls := iotaTestServer(t, http.StatusOK)
	defer server.Close()

	registry := &Registry{IotaAPI: platforms.NewIotaAPIService(server.URL, "test-key", nil)}
	resolved, err := registry.resolveYouTube(
		context.Background(), "https://youtu.be/abc12345678",
	)
	if err != nil {
		t.Fatalf("resolve failed: %v", err)
	}
	if resolved.Backend != platforms.BackendIota {
		t.Fatalf("backend = %q, want %q", resolved.Backend, platforms.BackendIota)
	}
	if resolved.Post == nil || resolved.Post.ID != "yt:abc12345678" {
		t.Fatalf("post id = %+v, want yt:abc12345678", resolved.Post)
	}
	if resolved.Post.Metadata.Title != "Track Title" {
		t.Fatalf("title = %q, want Track Title", resolved.Post.Metadata.Title)
	}
	if len(resolved.Post.Media) != 1 {
		t.Fatalf("media count = %d, want 1", len(resolved.Post.Media))
	}
	if !strings.Contains(resolved.Post.Media[0].URL, "/download/yt:abc12345678") {
		t.Fatalf("media url = %q, want the download endpoint", resolved.Post.Media[0].URL)
	}
	if resolved.Headers["X-API-Key"] != "test-key" {
		t.Fatalf("headers = %+v, want the api key", resolved.Headers)
	}
	if *infoCalls != 1 {
		t.Fatalf("info calls = %d, want 1", *infoCalls)
	}
}

// The metadata of the api can be blocked for a moment. The download
// endpoint resolves the stream on its own, so the video must still be
// delivered instead of failing the whole request.
func TestResolveYouTubeSurvivesAMetadataFailure(t *testing.T) {
	server, _ := iotaTestServer(t, http.StatusServiceUnavailable)
	defer server.Close()

	registry := &Registry{IotaAPI: platforms.NewIotaAPIService(server.URL, "test-key", nil)}
	resolved, err := registry.resolveYouTube(
		context.Background(), "https://youtu.be/abc12345678",
	)
	if err != nil {
		t.Fatalf("resolve failed: %v", err)
	}
	if resolved.Backend != platforms.BackendIota {
		t.Fatalf("backend = %q, want %q", resolved.Backend, platforms.BackendIota)
	}
	if resolved.Post.ID != "yt:abc12345678" {
		t.Fatalf("post id = %q, want yt:abc12345678", resolved.Post.ID)
	}
	if resolved.Post.Metadata.Title == "" {
		t.Fatal("the fallback title must not be empty")
	}
}

// The own yt-dlp resolver must stay the fallback of the api: an api
// that cannot be reached must not end the resolution.
func TestResolveYouTubeFallsBackToTheOwnResolver(t *testing.T) {
	// A closed port: the api cannot be reached at all.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	url := server.URL
	server.Close()

	registry := &Registry{IotaAPI: platforms.NewIotaAPIService(url, "test-key", nil)}
	_, err := registry.resolveYouTube(
		context.Background(), "https://youtu.be/abc12345678",
	)
	if err == nil {
		t.Fatal("the resolution must continue with the own resolver")
	}
	// The own resolver is not configured in the test, so its own error
	// is returned. An api error would name the api instead.
	if !strings.Contains(err.Error(), "youtube service is not ready") {
		t.Fatalf("error = %v, want the fallback of the own resolver", err)
	}
}

// The download url of the api carries the height of the user setting.
func TestIotaDownloadURLCarriesQuality(t *testing.T) {
	server, _ := iotaTestServer(t, http.StatusOK)
	defer server.Close()

	api := platforms.NewIotaAPIService(server.URL, "test-key", nil)

	url := api.DownloadURL("yt:abc12345678", platforms.KindVideo, 720, 0, "")
	if !strings.Contains(url, "kind=video") || !strings.Contains(url, "max_height=720") {
		t.Fatalf("url = %q, want kind=video and max_height=720", url)
	}

	audio := api.DownloadURL("yt:abc12345678", platforms.KindAudio, 0, 192, "mp3")
	for _, want := range []string{"kind=audio", "bitrate=192", "format=mp3"} {
		if !strings.Contains(audio, want) {
			t.Fatalf("url = %q, want %s", audio, want)
		}
	}
}
