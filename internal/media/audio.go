package media

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

// EnsureAudioFormat converts an audio file into the requested container when
// it does not match it already and returns the final path.
//
// A missing ffmpeg leaves the file untouched.
func EnsureAudioFormat(ctx context.Context, path, codec string, bitrate int) (string, error) {
	return ConvertAudioFormat(ctx, path, codec, bitrate, false)
}

// ConvertAudioFormat converts an audio file into the requested container and
// bitrate.
//
// A file that already has the wanted container is only converted when force
// is set, the file of a music api comes with a bitrate of its own.
func ConvertAudioFormat(ctx context.Context, path, codec string, bitrate int, force bool) (string, error) {
	if path == "" {
		return path, nil
	}

	codec = strings.ToLower(codec)
	if codec == "" {
		return path, nil
	}
	if !force && strings.EqualFold(strings.TrimPrefix(filepath.Ext(path), "."), codec) {
		return path, nil
	}
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		return path, nil
	}

	target := strings.TrimSuffix(path, filepath.Ext(path)) + "." + codec
	sameFile := target == path
	if sameFile {
		// The conversion must not read and write the same file.
		target = path + ".converted." + codec
	}
	args := []string{"-y", "-i", path, "-vn"}
	switch codec {
	case "mp3":
		args = append(args, "-c:a", "libmp3lame")
		if bitrate > 0 {
			args = append(args, "-b:a", strconv.Itoa(bitrate)+"k")
		}
	case "m4a":
		args = append(args, "-c:a", "aac")
		if bitrate > 0 {
			args = append(args, "-b:a", strconv.Itoa(bitrate)+"k")
		}
	case "flac":
		args = append(args, "-c:a", "flac")
	default:
		args = append(args, "-c:a", "copy")
	}
	args = append(args, target)

	command := exec.CommandContext(ctx, "ffmpeg", args...)
	if err := command.Run(); err != nil {
		_ = os.Remove(target)
		return path, fmt.Errorf("convert audio to %s: %w", codec, err)
	}
	if sameFile {
		if err := os.Rename(target, path); err != nil {
			return path, fmt.Errorf("replace the audio file: %w", err)
		}
		return path, nil
	}
	return target, nil
}

// EmbedAudioCover writes the cover image into the audio file with ffmpeg.
//
// The container of the source file is kept, so an mp3 stays an mp3 and a m4a
// stays a m4a.
func EmbedAudioCover(ctx context.Context, audioPath, coverPath string) (string, bool) {
	if audioPath == "" || coverPath == "" {
		return "", false
	}
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		return "", false
	}
	if _, err := os.Stat(coverPath); err != nil {
		return "", false
	}

	extension := strings.ToLower(filepath.Ext(audioPath))
	if extension == "" {
		extension = ".mp3"
	}
	target := strings.TrimSuffix(audioPath, filepath.Ext(audioPath)) + ".cover" + extension

	args := []string{
		"-y",
		"-i", audioPath,
		"-i", coverPath,
		"-map", "0:a",
		"-map", "1:v",
		"-c:a", "copy",
		"-c:v", "mjpeg",
		"-disposition:v:0", "attached_pic",
	}
	if extension == ".mp3" {
		args = append(args, "-id3v2_version", "3")
	}
	args = append(args, target)

	command := exec.CommandContext(ctx, "ffmpeg", args...)
	if err := command.Run(); err != nil {
		_ = os.Remove(target)
		return "", false
	}
	return target, true
}
