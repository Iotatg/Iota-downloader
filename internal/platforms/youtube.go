package platforms

import (
	"context"
	"fmt"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

// YouTubeService resolves YouTube and YouTube Music links.
type YouTubeService struct {
	ytdlp     *YTDLP
	outputDir string
	log       Logger
}

// NewYouTubeService builds the YouTube resolver.
func NewYouTubeService(ytdlp *YTDLP, outputDir string, log Logger) *YouTubeService {
	return &YouTubeService{ytdlp: ytdlp, outputDir: outputDir, log: log}
}

// YouTubeInfo is the normalised metadata of a YouTube video.
type YouTubeInfo struct {
	ID          string
	Title       string
	WebpageURL  string
	Duration    int
	Uploader    string
	Artist      string
	Thumbnail   string
	ViewCount   int64
	LikeCount   int64
	ReleaseDate string
	UploadDate  string
	VideoStream map[string]any
	AudioStream map[string]any
}

// FetchVideo resolves a YouTube video into a post.
func (s *YouTubeService) FetchVideo(ctx context.Context, url string) (*Post, error) {
	info, err := s.ytdlp.ExtractInfo(ctx, url)
	if err != nil {
		return nil, err
	}
	return s.PostFromInfo(info), nil
}

// FetchVideoWithInfo resolves a video and hands the raw yt-dlp data over.
func (s *YouTubeService) FetchVideoWithInfo(ctx context.Context, url string) (*Post, map[string]any, error) {
	info, err := s.ytdlp.ExtractInfo(ctx, url)
	if err != nil {
		return nil, nil, err
	}
	return s.PostFromInfo(info), info, nil
}

// PostFromInfo converts raw yt-dlp data into a post.
func (s *YouTubeService) PostFromInfo(info map[string]any) *Post {
	parsed := ParseYouTubeInfo(info)

	post := &Post{
		ID:          parsed.ID,
		Service:     ServiceYouTube,
		Author:      parsed.Uploader,
		Description: parsed.Title,
		Metadata: &Metadata{
			Title:     parsed.Title,
			Artists:   parsed.Artist,
			Duration:  secondsToDuration(parsed.Duration),
			Thumbnail: parsed.Thumbnail,
			SourceURL: parsed.WebpageURL,
			URL:       parsed.WebpageURL,
		},
	}

	if parsed.VideoStream != nil {
		streamURL, _ := parsed.VideoStream["url"].(string)
		if streamURL != "" {
			post.Media = append(post.Media, Media{
				URL:   streamURL,
				Type:  KindVideo,
				Thumb: parsed.Thumbnail,
			})
		}
	}
	if len(post.Media) == 0 {
		post.Media = append(post.Media, Media{URL: parsed.WebpageURL, Type: KindVideo, Thumb: parsed.Thumbnail})
	}
	return post
}

// ParseYouTubeInfo normalises the yt-dlp answer of a video.
func ParseYouTubeInfo(info map[string]any) *YouTubeInfo {
	parsed := &YouTubeInfo{
		ID:          stringValue(info["id"]),
		Title:       stringValue(info["title"]),
		WebpageURL:  stringValue(info["webpage_url"]),
		Uploader:    stringValue(info["uploader"]),
		Duration:    intValue(info["duration"]),
		ViewCount:   int64Value(info["view_count"]),
		LikeCount:   int64Value(info["like_count"]),
		ReleaseDate: stringValue(info["release_date"]),
		UploadDate:  stringValue(info["upload_date"]),
	}
	if parsed.WebpageURL == "" {
		parsed.WebpageURL = stringValue(info["original_url"])
	}
	if parsed.Uploader == "" {
		parsed.Uploader = stringValue(info["creator"])
	}
	parsed.Artist = NormalizeArtistNames(info["artists"])
	if parsed.Artist == "" {
		parsed.Artist = parsed.Uploader
	}
	parsed.Thumbnail = YouTubeThumbnail(info)
	parsed.VideoStream = SelectVideoStream(info, 1080)
	parsed.AudioStream = SelectAudioStream(info)
	return parsed
}

// PhotoMediaFromInfo returns the photo of a yt-dlp answer.
//
// A post without a video stream is a photo post, the file of the platform is
// then fetched directly.
func PhotoMediaFromInfo(info map[string]any) (string, string) {
	if info == nil {
		return "", ""
	}
	if formats, ok := info["formats"].([]any); ok && len(formats) > 0 {
		return "", ""
	}
	switch strings.ToLower(stringValue(info["ext"])) {
	case "jpg", "jpeg", "png", "webp", "gif":
	default:
		return "", ""
	}
	url := stringValue(info["url"])
	if url == "" {
		return "", ""
	}
	return KindPhoto, url
}

// YouTubeThumbnail picks the best thumbnail of a video.
func YouTubeThumbnail(info map[string]any) string {
	thumbnails, ok := info["thumbnails"].([]any)
	if ok && len(thumbnails) > 0 {
		type candidate struct {
			area   int
			isJPEG bool
			index  int
			url    string
		}
		var best candidate
		for index, item := range thumbnails {
			entry, ok := item.(map[string]any)
			if !ok {
				continue
			}
			width := intValue(entry["width"])
			height := intValue(entry["height"])
			url := stringValue(entry["url"])
			if url == "" {
				continue
			}
			current := candidate{area: width * height, isJPEG: strings.Contains(strings.ToLower(url), ".jpg") || strings.Contains(strings.ToLower(url), ".jpeg"), index: index, url: url}
			if best.url == "" || current.area > best.area ||
				(current.area == best.area && current.isJPEG && !best.isJPEG) {
				best = current
			}
		}
		if best.url != "" {
			return best.url
		}
	}
	if thumbnail := stringValue(info["thumbnail"]); thumbnail != "" {
		return thumbnail
	}
	if id := stringValue(info["id"]); id != "" {
		return "https://i.ytimg.com/vi/" + id + "/hqdefault.jpg"
	}
	return ""
}

// SelectVideoStream returns the best progressive mp4 stream of a video.
func SelectVideoStream(info map[string]any, maxHeight int) map[string]any {
	formats, ok := info["formats"].([]any)
	if !ok {
		return nil
	}
	var progressive []map[string]any
	for _, item := range formats {
		entry, ok := item.(map[string]any)
		if !ok {
			continue
		}
		if stringValue(entry["vcodec"]) == "none" || stringValue(entry["acodec"]) == "none" {
			continue
		}
		if stringValue(entry["ext"]) != "mp4" {
			continue
		}
		height := intValue(entry["height"])
		if height <= 0 || height > maxHeight {
			continue
		}
		progressive = append(progressive, entry)
	}
	if len(progressive) == 0 {
		return nil
	}
	sort.SliceStable(progressive, func(i, j int) bool {
		return intValue(progressive[i]["height"]) > intValue(progressive[j]["height"])
	})
	chosen := progressive[0]
	if info != nil {
		if webpage, ok := info["webpage_url"].(string); ok {
			chosen["webpage_url"] = webpage
		}
	}
	return chosen
}

// SelectAudioStream returns the best audio only stream of a video.
func SelectAudioStream(info map[string]any) map[string]any {
	formats, ok := info["formats"].([]any)
	if !ok {
		return nil
	}
	var audioOnly []map[string]any
	for _, item := range formats {
		entry, ok := item.(map[string]any)
		if !ok {
			continue
		}
		if stringValue(entry["vcodec"]) != "none" {
			continue
		}
		ext := stringValue(entry["ext"])
		if ext != "m4a" && ext != "mp4" {
			continue
		}
		audioOnly = append(audioOnly, entry)
	}
	if len(audioOnly) == 0 {
		return nil
	}
	sort.SliceStable(audioOnly, func(i, j int) bool {
		return floatValue(audioOnly[i]["abr"]) > floatValue(audioOnly[j]["abr"])
	})
	return audioOnly[0]
}

// DownloadVideo downloads a video with yt-dlp using the given format selector.
func (s *YouTubeService) DownloadVideo(ctx context.Context, url, filename, formatSelector string, maxSize int64) (string, error) {
	template := filepath.Join(s.outputDir, filename)
	return s.ytdlp.Download(ctx, url, DownloadOptions{
		FormatSelector: formatSelector,
		OutputTemplate: template,
		MergeFormat:    "mp4",
		MaxFilesize:    maxSize,
	})
}

// DownloadAudio downloads the audio track of a video in the requested format.
//
// The codec is one of mp3, m4a and flac. A bitrate of zero keeps the quality
// of the source, which is used by the lossless containers.
func (s *YouTubeService) DownloadAudio(ctx context.Context, url, baseName string, maxSize int64, codec string, bitrate int) (string, error) {
	template := filepath.Join(s.outputDir, baseName+".%(ext)s")
	if codec == "" {
		codec = "mp3"
	}

	options := DownloadOptions{
		FormatSelector: "bestaudio[ext=m4a]/bestaudio/best",
		OutputTemplate: template,
		MaxFilesize:    maxSize,
		ExtractAudio:   true,
		AudioCodec:     codec,
	}
	switch codec {
	case "mp3":
		if bitrate <= 0 {
			bitrate = 320
		}
		options.AudioQuality = strconv.Itoa(bitrate) + "K"
	case "flac":
		options.AudioQuality = "0"
	}

	return s.ytdlp.Download(ctx, url, options)
}

// DownloadStream resolves the direct URL and the headers of a media stream.
func (s *YouTubeService) DownloadStream(stream map[string]any) (string, map[string]string, error) {
	streamURL := stringValue(stream["url"])
	if streamURL == "" {
		return "", nil, fmt.Errorf("stream without url")
	}
	headers := map[string]string{}
	if raw, ok := stream["http_headers"].(map[string]any); ok {
		for key, value := range raw {
			if text, ok := value.(string); ok {
				headers[key] = text
			}
		}
	}
	return streamURL, headers, nil
}

func secondsToDuration(seconds int) time.Duration {
	if seconds <= 0 {
		return 0
	}
	return time.Duration(seconds) * time.Second
}

func stringValue(value any) string {
	if value == nil {
		return ""
	}
	switch typed := value.(type) {
	case string:
		return typed
	case float64:
		return strconv.FormatFloat(typed, 'f', -1, 64)
	case int:
		return strconv.Itoa(typed)
	case int64:
		return strconv.FormatInt(typed, 10)
	case bool:
		return strconv.FormatBool(typed)
	default:
		return fmt.Sprint(typed)
	}
}

func intValue(value any) int {
	switch typed := value.(type) {
	case float64:
		return int(typed)
	case int:
		return typed
	case int64:
		return int(typed)
	case string:
		parsed, err := strconv.Atoi(typed)
		if err == nil {
			return parsed
		}
	}
	return 0
}

func int64Value(value any) int64 {
	switch typed := value.(type) {
	case float64:
		return int64(typed)
	case int:
		return int64(typed)
	case int64:
		return typed
	case string:
		parsed, err := strconv.ParseInt(typed, 10, 64)
		if err == nil {
			return parsed
		}
	}
	return 0
}

func floatValue(value any) float64 {
	switch typed := value.(type) {
	case float64:
		return typed
	case int:
		return float64(typed)
	case int64:
		return float64(typed)
	case string:
		parsed, err := strconv.ParseFloat(typed, 64)
		if err == nil {
			return parsed
		}
	}
	return 0
}
