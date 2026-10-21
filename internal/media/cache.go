// Package media implements the delivery pipeline of the Iota Coder bot: the
// cache aware orchestration of downloads, the media group handling and the
// audio metadata preparation.
package media

import (
	"fmt"
	"strings"
)

// AudioCacheVariant marks the audio cache entries that carry the metadata of
// the artist.
const AudioCacheVariant = "audio_artist_dedupe_v6"

// YouTubeVideoCacheVariant marks the cache entries of the videos that were
// downloaded with the format selector of the quality setting.
//
// The variant is part of the key, so a file that was downloaded with an older
// selector is not reused when the selector changes.
const YouTubeVideoCacheVariant = "quality_selector_v2"

// BuildMediaCacheKey assembles the cache key of a media item.
//
// The key is built from a stable base identifier and may carry the index of
// the item inside an album, its kind and a variant suffix.
func BuildMediaCacheKey(base string, variant string, itemIndex *int, itemKind string) (string, error) {
	base = strings.TrimSpace(base)
	if base == "" {
		return "", fmt.Errorf("empty media cache key")
	}

	key := base
	if itemIndex != nil {
		key = fmt.Sprintf("%s#item:%d:%s", key, *itemIndex, itemKind)
	}
	if variant != "" {
		key = key + "#" + variant
	}
	return key, nil
}

// AudioCacheKey returns the cache key of an audio track.
//
// The format is part of the key, so a track that was downloaded as mp3 is not
// reused when the user asks for flac.
func AudioCacheKey(sourceURL string, format string) string {
	key := sourceURL + "#" + AudioCacheVariant
	if format != "" {
		key += "#f:" + format
	}
	return key
}

// YouTubeVideoCacheKey returns the cache key of a YouTube video.
//
// The quality is part of the key, so a video that was downloaded in 480p is
// not reused when the user asks for 4K. The variant marks the entries that
// were downloaded with the current format selector.
func YouTubeVideoCacheKey(webpageURL string, quality string) string {
	key := webpageURL
	if quality != "" {
		key += "#q:" + quality
	}
	return key + "#" + YouTubeVideoCacheVariant
}

// YouTubeDocumentCacheKey returns the cache key of the original YouTube file.
func YouTubeDocumentCacheKey(webpageURL string) string {
	return webpageURL + "#document"
}

// TikTokVideoCacheKey returns the cache key of a TikTok video.
func TikTokVideoCacheKey(videoID string) string {
	return "tiktok#video:" + videoID
}

// TikTokDocumentCacheKey returns the cache key of the original TikTok file.
func TikTokDocumentCacheKey(videoID string) string {
	return "tiktok#doc:" + videoID
}

// TwitterMediaCacheKey returns the cache key of a tweet media item.
func TwitterMediaCacheKey(postURL string, index int, kind string, total int) string {
	if total == 1 && kind == "video" {
		return postURL
	}
	return fmt.Sprintf("%s#item:%d:%s", postURL, index, kind)
}

// InstagramAudioCacheKey returns the cache key of an Instagram audio track.
func InstagramAudioCacheKey(sourceURL string) string {
	return sourceURL + "#audio"
}
