package platforms

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"strings"
	"time"

	"github.com/Iotatg/iota-downloader/internal/httpx"
)

// CobaltClient talks to a Cobalt compatible download backend.
type CobaltClient struct {
	BaseURL string
	APIKey  string
	Client  *http.Client
	Log     Logger
}

// Logger is the minimal logging surface of the platform services.
type Logger interface {
	Info(format string, args ...any)
	Warning(format string, args ...any)
	Error(format string, args ...any)
	Debug(format string, args ...any)
	Perf(name string, durationMS float64, fields map[string]any)
}

// CobaltResponse is the raw answer of a Cobalt backend.
type CobaltResponse struct {
	Status   string          `json:"status"`
	URL      string          `json:"url"`
	Filename string          `json:"filename"`
	Picker   []CobaltPicker  `json:"picker"`
	Tunnel   json.RawMessage `json:"tunnel"`
	Audio    string          `json:"audio"`
	Type     string          `json:"type"`
	Metadata json.RawMessage `json:"metadata"`
	Output   *CobaltOutput   `json:"output"`
	Error    *CobaltError    `json:"error"`
}

// CobaltPicker is one entry of a picker answer.
type CobaltPicker struct {
	URL   string `json:"url"`
	Type  string `json:"type"`
	Thumb string `json:"thumb"`
}

// CobaltOutput carries the result of a local processing job.
type CobaltOutput struct {
	Filename string          `json:"filename"`
	Type     string          `json:"type"`
	Metadata json.RawMessage `json:"metadata"`
}

// CobaltError describes a backend failure.
type CobaltError struct {
	Code    string `json:"code"`
	Context string `json:"context"`
}

// CobaltItem is a single media file returned by Cobalt.
type CobaltItem struct {
	URL   string
	Type  string
	Thumb string
}

// Fetch performs a Cobalt request with retries.
func (c *CobaltClient) Fetch(ctx context.Context, payload map[string]any, source string) (*CobaltResponse, error) {
	if c == nil || c.BaseURL == "" || c.APIKey == "" {
		return nil, fmt.Errorf("cobalt backend is not configured")
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal cobalt payload: %w", err)
	}

	endpoint := strings.TrimRight(c.BaseURL, "/") + "/"
	attempts := 3
	var lastErr error

	for attempt := 1; attempt <= attempts; attempt++ {
		started := time.Now()
		request, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
		if err != nil {
			return nil, fmt.Errorf("build cobalt request: %w", err)
		}
		request.Header.Set("Accept", "application/json")
		request.Header.Set("Content-Type", "application/json")
		request.Header.Set("Authorization", "Api-Key "+strings.Trim(c.APIKey, "\""))

		response, err := c.Client.Do(request)
		if err != nil {
			lastErr = fmt.Errorf("cobalt request failed: %w", err)
			if attempt < attempts {
				if err := sleepBackoff(ctx, attempt); err != nil {
					return nil, err
				}
				continue
			}
			break
		}

		raw, readErr := io.ReadAll(io.LimitReader(response.Body, 8<<20))
		response.Body.Close()
		if readErr != nil {
			lastErr = fmt.Errorf("read cobalt response: %w", readErr)
			if attempt < attempts {
				if err := sleepBackoff(ctx, attempt); err != nil {
					return nil, err
				}
				continue
			}
			break
		}

		if c.Log != nil {
			c.Log.Perf("cobalt_request", float64(time.Since(started).Milliseconds()), map[string]any{
				"source": source,
				"status": response.StatusCode,
			})
		}

		if response.StatusCode >= 500 {
			lastErr = fmt.Errorf("cobalt returned %d", response.StatusCode)
			if attempt < attempts {
				if err := sleepBackoff(ctx, attempt); err != nil {
					return nil, err
				}
				continue
			}
			break
		}

		var parsed CobaltResponse
		if err := json.Unmarshal(raw, &parsed); err != nil {
			lastErr = fmt.Errorf("decode cobalt response: %w", err)
			if response.StatusCode >= 500 && attempt < attempts {
				if err := sleepBackoff(ctx, attempt); err != nil {
					return nil, err
				}
				continue
			}
			break
		}

		if parsed.Error != nil && parsed.Error.Code == "error.api.fetch.empty" && attempt < attempts {
			lastErr = fmt.Errorf("cobalt: %s", parsed.Error.Code)
			if err := sleepBackoff(ctx, attempt); err != nil {
				return nil, err
			}
			continue
		}
		return &parsed, nil
	}
	return nil, lastErr
}

func sleepBackoff(ctx context.Context, attempt int) error {
	delay := time.Second * time.Duration(1<<(attempt-1))
	delay += time.Duration(rand.Intn(1000)) * time.Millisecond
	if delay > 30*time.Second {
		delay = 30 * time.Second
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(delay):
		return nil
	}
}

// ParseItems converts a Cobalt answer into the list of media files.
func ParseItems(response *CobaltResponse, audioOnly bool, allowMultiTunnel bool) ([]CobaltItem, error) {
	if response == nil {
		return nil, fmt.Errorf("empty cobalt response")
	}
	status := response.Status
	if status == "" {
		if response.URL != "" {
			status = "tunnel"
		} else if len(response.Picker) > 0 {
			status = "picker"
		}
	}

	switch status {
	case "tunnel", "redirect":
		if response.URL == "" {
			return nil, fmt.Errorf("cobalt response without url")
		}
		kind := classifyMedia(response.URL, audioOnly, "", response.Filename, "")
		return []CobaltItem{{URL: response.URL, Type: kind, Thumb: extractThumb(response.Metadata, response.Output)}}, nil

	case "picker":
		if audioOnly {
			if response.Audio == "" {
				return nil, fmt.Errorf("cobalt picker without audio")
			}
			return []CobaltItem{{URL: response.Audio, Type: KindAudio}}, nil
		}
		items := make([]CobaltItem, 0, len(response.Picker))
		for _, picker := range response.Picker {
			if picker.URL == "" {
				continue
			}
			kind := picker.Type
			if kind == "" {
				kind = classifyMedia(picker.URL, false, "", "", "")
			}
			items = append(items, CobaltItem{URL: picker.URL, Type: kind, Thumb: picker.Thumb})
		}
		if len(items) == 0 {
			return nil, fmt.Errorf("cobalt picker without items")
		}
		return items, nil

	case "local-processing":
		var tunnels []string
		if len(response.Tunnel) > 0 {
			if err := json.Unmarshal(response.Tunnel, &tunnels); err != nil {
				return nil, fmt.Errorf("decode cobalt tunnel list: %w", err)
			}
		}
		if !allowMultiTunnel && !audioOnly && len(tunnels) > 1 {
			return nil, fmt.Errorf("cobalt returned %d files for a single media request", len(tunnels))
		}
		filename := ""
		declared := response.Type
		if response.Output != nil {
			filename = response.Output.Filename
			if response.Output.Type != "" {
				declared = response.Output.Type
			}
		}
		items := make([]CobaltItem, 0, len(tunnels))
		for _, tunnel := range tunnels {
			if tunnel == "" {
				continue
			}
			kind := classifyMedia(tunnel, audioOnly, declared, filename, "")
			items = append(items, CobaltItem{URL: tunnel, Type: kind, Thumb: extractThumb(response.Metadata, response.Output)})
		}
		if len(items) == 0 {
			return nil, fmt.Errorf("cobalt local processing without files")
		}
		return items, nil

	case "error":
		if response.Error != nil {
			return nil, fmt.Errorf("cobalt error %s: %s", response.Error.Code, response.Error.Context)
		}
		return nil, fmt.Errorf("cobalt returned an error")

	default:
		return nil, fmt.Errorf("unknown cobalt status %q", status)
	}
}

func classifyMedia(mediaURL string, audioOnly bool, declaredType, filename, mimeType string) string {
	if audioOnly {
		return KindAudio
	}
	switch strings.ToLower(declaredType) {
	case "video", "gif", "merge", "mute", "remux":
		return KindVideo
	case "photo":
		return KindPhoto
	case "audio":
		return KindAudio
	}
	if strings.HasPrefix(mimeType, "video/") {
		return KindVideo
	}
	if strings.HasPrefix(mimeType, "image/") {
		return KindPhoto
	}
	if strings.HasPrefix(mimeType, "audio/") {
		return KindAudio
	}

	probe := strings.ToLower(mediaURL + " " + filename)
	switch {
	case containsAny(probe, ".mp3", ".m4a", ".aac", ".ogg", ".wav", ".opus"):
		return KindAudio
	case containsAny(probe, ".jpg", ".jpeg", ".png", ".webp", ".avif"):
		return KindPhoto
	default:
		return KindVideo
	}
}

func containsAny(value string, needles ...string) bool {
	for _, needle := range needles {
		if strings.Contains(value, needle) {
			return true
		}
	}
	return false
}

func extractThumb(metadata json.RawMessage, output *CobaltOutput) string {
	values := []json.RawMessage{}
	if len(metadata) > 0 {
		values = append(values, metadata)
	}
	if output != nil && len(output.Metadata) > 0 {
		values = append(values, output.Metadata)
	}

	for _, raw := range values {
		var parsed map[string]any
		if err := json.Unmarshal(raw, &parsed); err != nil {
			continue
		}
		for _, key := range []string{"thumb", "thumbnail", "poster", "preview", "cover"} {
			if value, ok := parsed[key].(string); ok && value != "" {
				return value
			}
		}
	}
	return ""
}

// NewCobaltClient builds a Cobalt client.
func NewCobaltClient(baseURL, apiKey string, log Logger) *CobaltClient {
	return &CobaltClient{
		BaseURL: baseURL,
		APIKey:  apiKey,
		Client:  httpx.NewClient(20*time.Second, 32),
		Log:     log,
	}
}
