package media

import (
	"os"
	"path/filepath"
	"strings"
)

// ffmpegBinary returns the ffmpeg binary the media helpers use.
//
// The PATH of a server can point at a build that crashes on a fragmented
// mp4. The folder of a known good build wins over it when FFMPEG_LOCATION
// names one.
func ffmpegBinary() string {
	folder := strings.TrimSpace(os.Getenv("FFMPEG_LOCATION"))
	if folder != "" {
		candidate := filepath.Join(folder, "ffmpeg")
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			return candidate
		}
	}
	return "ffmpeg"
}
