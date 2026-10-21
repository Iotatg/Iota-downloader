package platforms

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
)

// PlaylistEntry is one entry of a flat playlist listing.
type PlaylistEntry struct {
	ID       string
	Title    string
	URL      string
	Uploader string
	Duration int
	Thumb    string
}

// PlaylistInfo is the flat listing of a playlist.
//
// The listing holds the entries of a playlist without the streams of the
// entries, it is fast enough to answer in a second.
type PlaylistInfo struct {
	ID      string
	Title   string
	Entries []PlaylistEntry
}

// ExtractPlaylist returns the entries of a playlist as reported by yt-dlp.
//
// The flat mode of yt-dlp only reads the list of a playlist, it never asks
// for the streams of the entries.
func (y *YTDLP) ExtractPlaylist(ctx context.Context, url string) (*PlaylistInfo, error) {
	if !y.Available() {
		return nil, fmt.Errorf("yt-dlp is not installed")
	}

	infoCtx, cancel := context.WithTimeout(ctx, y.opts.InfoTimeout)
	defer cancel()

	args := append(y.baseArgsWithPlaylist(true), "--flat-playlist", "--dump-single-json", "--no-check-certificate", url)
	cmd := exec.CommandContext(infoCtx, y.opts.Binary, args...)
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("yt-dlp playlist failed for %s: %w", url, err)
	}

	var raw map[string]any
	if err := json.Unmarshal(output, &raw); err != nil {
		return nil, fmt.Errorf("decode yt-dlp playlist: %w", err)
	}
	return PlaylistFromInfo(raw), nil
}

// PlaylistFromInfo reads the flat listing of a playlist out of the raw data
// of yt-dlp.
func PlaylistFromInfo(info map[string]any) *PlaylistInfo {
	if info == nil {
		return nil
	}
	kind, _ := info["_type"].(string)
	if kind != "playlist" {
		return nil
	}

	playlist := &PlaylistInfo{
		ID:    stringValue(info["id"]),
		Title: stringValue(info["title"]),
	}

	entries, _ := info["entries"].([]any)
	for _, entry := range entries {
		item, ok := entry.(map[string]any)
		if !ok {
			// A private entry of a playlist carries no data.
			continue
		}
		link := stringValue(item["url"])
		if link == "" {
			continue
		}
		playlist.Entries = append(playlist.Entries, PlaylistEntry{
			ID:       stringValue(item["id"]),
			Title:    stringValue(item["title"]),
			URL:      link,
			Uploader: stringValue(item["uploader"]),
			Duration: intValue(item["duration"]),
			Thumb:    bestThumbnail(item["thumbnails"]),
		})
	}
	return playlist
}

// bestThumbnail returns the largest cover of a list of thumbnails.
func bestThumbnail(value any) string {
	items, ok := value.([]any)
	if !ok {
		return ""
	}
	best := ""
	bestWidth := -1
	for _, item := range items {
		thumb, ok := item.(map[string]any)
		if !ok {
			continue
		}
		link := stringValue(thumb["url"])
		if link == "" {
			continue
		}
		width := intValue(thumb["width"])
		if width > bestWidth {
			best = link
			bestWidth = width
		}
	}
	return best
}

// CollectionFromPlaylist turns the flat listing of yt-dlp into a collection.
func CollectionFromPlaylist(service string, info *PlaylistInfo, limit int) *Collection {
	if info == nil {
		return nil
	}
	collection := &Collection{
		Service: service,
		Kind:    CollectionPlaylist,
		ID:      info.ID,
		Title:   info.Title,
		Total:   len(info.Entries),
	}
	for index, entry := range info.Entries {
		if limit > 0 && index >= limit {
			break
		}
		collection.Items = append(collection.Items, CollectionItem{
			Index:    index,
			Title:    entry.Title,
			Artist:   entry.Uploader,
			URL:      entry.URL,
			Thumb:    entry.Thumb,
			Duration: entry.Duration,
		})
	}
	return collection
}

// PlaylistTitle cleans the title of a playlist, the mixes of YouTube carry a
// long suffix that does not belong to the name.
func PlaylistTitle(title string) string {
	title = strings.TrimSpace(title)
	if index := strings.Index(title, " - "); index > 0 && strings.HasPrefix(title, "Mix - ") {
		return strings.TrimSpace(title[index+3:])
	}
	return title
}
