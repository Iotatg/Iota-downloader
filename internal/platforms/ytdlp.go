package platforms

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// YTDLPOptions configures the yt-dlp invocations.
type YTDLPOptions struct {
	CookiesFile        string
	CookiesFromBrowser string
	PlayerClient       string
	POToken            string
	RemoteComponents   string
	JSRuntimes         string
	InfoTimeout        time.Duration
	SleepRequests      float64
	SleepInterval      float64
	MaxSleepInterval   float64
	Binary             string
}

// YTDLP runs the yt-dlp binary.
type YTDLP struct {
	opts YTDLPOptions
	log  Logger
}

// NewYTDLP builds a yt-dlp runner.
func NewYTDLP(opts YTDLPOptions, log Logger) *YTDLP {
	if opts.Binary == "" {
		opts.Binary = "yt-dlp"
	}
	if opts.InfoTimeout <= 0 {
		opts.InfoTimeout = 45 * time.Second
	}
	return &YTDLP{opts: opts, log: log}
}

// Available reports whether the yt-dlp binary can be executed.
func (y *YTDLP) Available() bool {
	if y == nil {
		return false
	}
	_, err := exec.LookPath(y.opts.Binary)
	return err == nil
}

func (y *YTDLP) baseArgs() []string {
	return y.baseArgsWithPlaylist(false)
}

// baseArgsWithPlaylist builds the arguments of a run, a run that reads a
// playlist must not carry the flag that keeps yt-dlp on a single video.
func (y *YTDLP) baseArgsWithPlaylist(allowPlaylist bool) []string {
	if y == nil {
		return nil
	}
	args := []string{
		"--quiet",
		"--no-warnings",
		"--no-progress",
		"--continue",
		"--force-overwrites",
		"--no-cache-dir",
		"--socket-timeout", "15",
		"--retries", "2",
		"--fragment-retries", "2",
		"--concurrent-fragments", "4",
	}
	if !allowPlaylist {
		args = append(args, "--no-playlist")
	}
	if y.opts.CookiesFile != "" {
		if _, err := os.Stat(y.opts.CookiesFile); err == nil {
			args = append(args, "--cookies", y.opts.CookiesFile)
		}
	}
	if y.opts.CookiesFromBrowser != "" {
		args = append(args, "--cookies-from-browser", y.opts.CookiesFromBrowser)
	}
	if y.opts.PlayerClient != "" {
		args = append(args, "--extractor-args", "youtube:player_client="+y.opts.PlayerClient)
	}
	if y.opts.POToken != "" {
		args = append(args, "--extractor-args", "youtube:po_token="+y.opts.POToken)
	}
	if y.opts.RemoteComponents != "" {
		args = append(args, "--remote-components", y.opts.RemoteComponents)
	}
	if y.opts.JSRuntimes != "" {
		args = append(args, "--js-runtimes", y.opts.JSRuntimes)
	}
	if y.opts.SleepRequests > 0 {
		args = append(args, "--sleep-requests", strconv.FormatFloat(y.opts.SleepRequests, 'f', -1, 64))
	}
	if y.opts.SleepInterval > 0 {
		args = append(args, "--sleep-interval", strconv.FormatFloat(y.opts.SleepInterval, 'f', -1, 64))
	}
	if y.opts.MaxSleepInterval > 0 {
		args = append(args, "--max-sleep-interval", strconv.FormatFloat(y.opts.MaxSleepInterval, 'f', -1, 64))
	}
	return args
}

// ExtractInfo returns the metadata of a URL as reported by yt-dlp.
func (y *YTDLP) ExtractInfo(ctx context.Context, url string) (map[string]any, error) {
	if !y.Available() {
		return nil, fmt.Errorf("yt-dlp is not installed")
	}

	infoCtx, cancel := context.WithTimeout(ctx, y.opts.InfoTimeout)
	defer cancel()

	args := append(y.baseArgs(), "--dump-single-json", "--no-check-certificate", url)
	cmd := exec.CommandContext(infoCtx, y.opts.Binary, args...)
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("yt-dlp info failed for %s: %w", url, err)
	}

	var info map[string]any
	if err := json.Unmarshal(output, &info); err != nil {
		return nil, fmt.Errorf("decode yt-dlp info: %w", err)
	}
	return info, nil
}

// ExtractInfoWithCookies returns the metadata of a URL as reported by yt-dlp
// while using an additional cookie file.
func (y *YTDLP) ExtractInfoWithCookies(ctx context.Context, url, cookiesFile string) (map[string]any, error) {
	if !y.Available() {
		return nil, fmt.Errorf("yt-dlp is not installed")
	}

	infoCtx, cancel := context.WithTimeout(ctx, y.opts.InfoTimeout)
	defer cancel()

	args := y.baseArgs()
	if cookiesFile != "" {
		if _, err := os.Stat(cookiesFile); err == nil {
			args = append(args, "--cookies", cookiesFile)
		}
	}
	args = append(args, "--dump-single-json", "--no-check-certificate", url)

	cmd := exec.CommandContext(infoCtx, y.opts.Binary, args...)
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("yt-dlp info failed for %s: %w", url, err)
	}

	var info map[string]any
	if err := json.Unmarshal(output, &info); err != nil {
		return nil, fmt.Errorf("decode yt-dlp info: %w", err)
	}
	return info, nil
}

// DownloadOptions tune a single yt-dlp download.
type DownloadOptions struct {
	FormatSelector string
	OutputTemplate string
	MergeFormat    string
	MaxFilesize    int64
	ExtractAudio   bool
	AudioCodec     string
	AudioQuality   string
}

// Download runs yt-dlp and returns the path of the produced file.
func (y *YTDLP) Download(ctx context.Context, url string, opts DownloadOptions) (string, error) {
	if !y.Available() {
		return "", fmt.Errorf("yt-dlp is not installed")
	}

	args := append(y.baseArgs(), "--no-check-certificate")
	if opts.FormatSelector != "" {
		args = append(args, "-f", opts.FormatSelector)
	}
	if opts.MaxFilesize > 0 {
		args = append(args, "--max-filesize", strconv.FormatInt(opts.MaxFilesize, 10))
	}
	if opts.ExtractAudio {
		codec := opts.AudioCodec
		if codec == "" {
			codec = "mp3"
		}
		args = append(args, "--extract-audio", "--audio-format", codec)
		if opts.AudioQuality != "" {
			args = append(args, "--audio-quality", opts.AudioQuality)
		}
	}
	if opts.MergeFormat != "" {
		args = append(args, "--merge-output-format", opts.MergeFormat)
	}
	if opts.OutputTemplate != "" {
		args = append(args, "-o", opts.OutputTemplate)
	}
	args = append(args, url)

	cmd := exec.CommandContext(ctx, y.opts.Binary, args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("yt-dlp download failed for %s: %w: %s", url, err, truncateOutput(string(output)))
	}
	return resolveDownloadedPath(opts.OutputTemplate, output)
}

func truncateOutput(value string) string {
	value = strings.TrimSpace(value)
	if len(value) > 400 {
		return value[len(value)-400:]
	}
	return value
}

// resolveDownloadedPath finds the file produced by yt-dlp.
func resolveDownloadedPath(template string, output []byte) (string, error) {
	extension := filepath.Ext(template)
	stem := strings.TrimSuffix(template, extension)

	if _, err := os.Stat(template); err == nil {
		return template, nil
	}

	matches, err := filepath.Glob(stem + "*" + extension)
	if err == nil && len(matches) > 0 {
		return matches[0], nil
	}
	matches, err = filepath.Glob(stem + ".*")
	if err == nil && len(matches) > 0 {
		return matches[0], nil
	}
	if len(output) > 0 {
		for _, line := range strings.Split(string(output), "\n") {
			line = strings.TrimSpace(line)
			if line == "" || !strings.HasPrefix(line, "/") {
				continue
			}
			if _, err := os.Stat(line); err == nil {
				return line, nil
			}
		}
	}
	return "", fmt.Errorf("yt-dlp output file missing: %s", template)
}

// SearchTrack looks up a single track by name and artist.
func (y *YTDLP) SearchTrack(ctx context.Context, query string) (map[string]any, error) {
	return y.ExtractInfo(ctx, "ytsearch1:"+query)
}
