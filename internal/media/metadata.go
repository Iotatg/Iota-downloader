package media

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

// VideoAttributes describes the geometry of a video file.
type VideoAttributes struct {
	Width             int
	Height            int
	SupportsStreaming bool
}

// ProbeVideoAttributes reads the geometry of a video with ffprobe.
func ProbeVideoAttributes(ctx context.Context, path string) VideoAttributes {
	attributes := VideoAttributes{SupportsStreaming: true}
	if path == "" {
		return attributes
	}
	if _, err := exec.LookPath("ffprobe"); err != nil {
		return attributes
	}

	command := exec.CommandContext(ctx, "ffprobe",
		"-v", "error",
		"-select_streams", "v:0",
		"-show_entries", "stream=width,height,sample_aspect_ratio,display_aspect_ratio",
		"-of", "json",
		path,
	)
	output, err := command.Output()
	if err != nil {
		return attributes
	}

	var parsed struct {
		Streams []struct {
			Width              int    `json:"width"`
			Height             int    `json:"height"`
			SampleAspectRatio  string `json:"sample_aspect_ratio"`
			DisplayAspectRatio string `json:"display_aspect_ratio"`
		} `json:"streams"`
	}
	if err := json.Unmarshal(output, &parsed); err != nil || len(parsed.Streams) == 0 {
		return attributes
	}

	stream := parsed.Streams[0]
	attributes.Width = stream.Width
	attributes.Height = stream.Height

	target := 0.0
	if stream.DisplayAspectRatio != "" {
		if num, den, ok := parseRatio(stream.DisplayAspectRatio); ok {
			target = num / den
		}
	}
	if target == 0 && stream.SampleAspectRatio != "" {
		if num, den, ok := parseRatio(stream.SampleAspectRatio); ok && stream.Height > 0 {
			target = (float64(stream.Width) / float64(stream.Height)) * (num / den)
		}
	}
	if target > 0 && attributes.Height > 0 {
		if target < 0.2 || target > 5.0 {
			target = 0
		} else {
			encoded := float64(attributes.Width) / float64(attributes.Height)
			if encoded > 0 && absFloat(encoded-target)/target > 0.015 {
				attributes.Width = int(float64(attributes.Height) * target)
			}
		}
	}
	return attributes
}

func parseRatio(value string) (float64, float64, bool) {
	parts := strings.SplitN(value, ":", 2)
	if len(parts) != 2 {
		return 0, 0, false
	}
	num, err := strconv.ParseFloat(parts[0], 64)
	if err != nil {
		return 0, 0, false
	}
	den, err := strconv.ParseFloat(parts[1], 64)
	if err != nil {
		return 0, 0, false
	}
	if den == 0 {
		return 0, 0, false
	}
	return num, den, true
}

func absFloat(value float64) float64 {
	if value < 0 {
		return -value
	}
	return value
}

// ProbeAudioDuration reads the duration of an audio file with ffprobe.
func ProbeAudioDuration(ctx context.Context, path string) (int, bool) {
	if path == "" {
		return 0, false
	}
	if _, err := exec.LookPath("ffprobe"); err != nil {
		return 0, false
	}

	command := exec.CommandContext(ctx, "ffprobe",
		"-v", "error",
		"-show_entries", "format=duration",
		"-of", "default=noprint_wrappers=1:nokey=1",
		path,
	)
	output, err := command.Output()
	if err != nil {
		return 0, false
	}
	seconds, err := strconv.ParseFloat(strings.TrimSpace(string(output)), 64)
	if err != nil || seconds <= 0 {
		return 0, false
	}
	return int(seconds), true
}

// PrepareAudioFile rewrites the audio file with the requested metadata using
// ffmpeg. It returns the path of the final file.
func PrepareAudioFile(ctx context.Context, sourcePath string, metadata Metadata) string {
	if sourcePath == "" {
		return sourcePath
	}
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		return sourcePath
	}

	target := filepath.Join(filepath.Dir(sourcePath), "tagged-"+filepath.Base(sourcePath))
	args := []string{"-y", "-i", sourcePath}
	if metadata.Title != "" {
		args = append(args, "-metadata", "title="+metadata.Title)
	}
	if metadata.Artists != "" {
		args = append(args, "-metadata", "artist="+metadata.Artists)
	}
	if metadata.Album != "" {
		args = append(args, "-metadata", "album="+metadata.Album)
	}
	if metadata.AlbumArtist != "" {
		args = append(args, "-metadata", "album_artist="+metadata.AlbumArtist)
	}
	if metadata.Date != "" {
		args = append(args, "-metadata", "date="+metadata.Date)
	}
	if metadata.TrackNumber != "" {
		args = append(args, "-metadata", "track="+metadata.TrackNumber)
	}
	if metadata.DiscNumber != "" {
		args = append(args, "-metadata", "disc="+metadata.DiscNumber)
	}
	if metadata.SourceURL != "" {
		args = append(args, "-metadata", "comment="+metadata.SourceURL)
	}

	switch strings.ToLower(filepath.Ext(sourcePath)) {
	case ".mp3":
		args = append(args, "-c", "copy", "-id3v2_version", "3")
	case ".m4a", ".mp4", ".aac":
		args = append(args, "-c", "copy", "-movflags", "use_metadata_tags")
	default:
		args = append(args, "-c", "copy")
	}
	args = append(args, target)

	command := exec.CommandContext(ctx, "ffmpeg", args...)
	if err := command.Run(); err != nil {
		_ = os.Remove(target)
		return sourcePath
	}
	return target
}

// Metadata describes the tags written into an audio file.
type Metadata struct {
	Title       string
	Artists     string
	Album       string
	AlbumArtist string
	Date        string
	TrackNumber string
	DiscNumber  string
	Duration    int
	Thumbnail   string
	SourceURL   string
}

// FormatBytes renders a byte count in a human friendly way.
func FormatBytes(size int64) string {
	const unit = 1024
	if size < unit {
		return fmt.Sprintf("%d B", size)
	}
	div, exp := int64(unit), 0
	for n := size / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(size)/float64(div), "KMGTPE"[exp])
}

// FormatDuration renders a duration in the m:ss form.
func FormatDuration(seconds int) string {
	if seconds <= 0 {
		return "0:00"
	}
	hours := seconds / 3600
	minutes := (seconds % 3600) / 60
	remainder := seconds % 60
	if hours > 0 {
		return fmt.Sprintf("%d:%02d:%02d", hours, minutes, remainder)
	}
	return fmt.Sprintf("%d:%02d", minutes, remainder)
}
