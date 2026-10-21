package handlers

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/Iotatg/iota-downloader/internal/chart"
	"github.com/Iotatg/iota-downloader/internal/messages"
	"github.com/Iotatg/iota-downloader/internal/platforms"
	"github.com/Iotatg/iota-downloader/internal/storage"
	"github.com/Iotatg/iota-downloader/internal/text"
)

// serviceMeta describes how a platform is shown in the statistics screen.
type serviceMeta struct {
	// Name is the display name of the platform.
	Name string
	// Emoji is the icon of the platform.
	Emoji string
	// Order controls the position of the platform in the legend.
	Order int
}

// serviceMetaList lists the platforms in the order of the legend.
var serviceMetaList = []serviceMeta{
	{Name: "Instagram", Emoji: "☚", Order: 0},
	{Name: "Threads", Emoji: "☛", Order: 1},
	{Name: "TikTok", Emoji: "☝", Order: 2},
	{Name: "YouTube", Emoji: "☞", Order: 3},
	{Name: "SoundCloud", Emoji: "◇", Order: 4},
	{Name: "Spotify", Emoji: "◉", Order: 5},
	{Name: "Pinterest", Emoji: "◆", Order: 6},
	{Name: "Twitter", Emoji: "☟", Order: 7},
	{Name: "Apple Music", Emoji: "◈", Order: 8},
	{Name: "JioSaavn", Emoji: "◐", Order: 9},
	{Name: "Deezer", Emoji: "◑", Order: 10},
	{Name: "Gaana", Emoji: "◐", Order: 11},
	{Name: "Tidal", Emoji: "◈", Order: 12},
	{Name: "Dailymotion", Emoji: "◓", Order: 13},
	{Name: "Bluesky", Emoji: "⬡", Order: 14},
	{Name: "Facebook", Emoji: "⬢", Order: 15},
	{Name: "Reddit", Emoji: "⬣", Order: 16},
	{Name: "Twitch", Emoji: "⬤", Order: 17},
	{Name: "Kick", Emoji: "⬥", Order: 18},
	{Name: "Bilibili", Emoji: "⬦", Order: 19},
	{Name: "Tumblr", Emoji: "⬧", Order: 20},
	{Name: "LinkedIn", Emoji: "⬨", Order: 21},
	{Name: "IMDb", Emoji: "⯃", Order: 22},
	{Name: "MX Player", Emoji: "⬔", Order: 23},
	{Name: "Snapchat", Emoji: "⬕", Order: 24},
	{Name: "Mastodon", Emoji: "⬖", Order: 25},
	{Name: "Resso", Emoji: "⬗", Order: 26},
	{Name: "Other", Emoji: "▫", Order: 27},
}

// otherServiceOrder is the position of the platforms that the bot does not
// know in the legend.
const otherServiceOrder = 27

// serviceMetaOf returns the display information of a platform.
func serviceMetaOf(service string) serviceMeta {
	normalized := strings.ToLower(strings.TrimSpace(service))
	switch normalized {
	case "x":
		return serviceMeta{Name: "Twitter", Emoji: "☟", Order: 7}
	case "youtube music", "youtubemusic", "yt music", "youtube_music", "youtube_audio":
		return serviceMeta{Name: "YouTube", Emoji: "☞", Order: 3}
	}
	for _, meta := range serviceMetaList {
		if strings.ToLower(meta.Name) == normalized {
			return meta
		}
	}

	// The history keeps the identifier of a platform, the label of the
	// platform carries its name.
	label := platforms.ServiceLabel(normalized)
	if label != normalized {
		for _, meta := range serviceMetaList {
			if strings.ToLower(meta.Name) == strings.ToLower(label) {
				return meta
			}
		}
	}
	return serviceMeta{Name: "Other", Emoji: "▫", Order: otherServiceOrder}
}

// statsPayload holds everything the statistics screen shows.
type statsPayload struct {
	Period string
	Mode   string
	Total  int
	Peak   int
	PeakAt string
	Avg    float64
	Points []chart.Point
	Series []chart.Series
	Users  int
	Active int
	Groups int
	All    int
	Cached int
	Top    []topService
}

// topService is one entry of the platform ranking.
type topService struct {
	Name  string
	Emoji string
	Count int
	Share float64
}

// buildStatsPayload collects the data of the statistics screen.
func (h *Handler) buildStatsPayload(ctx context.Context, period, mode string) (*statsPayload, error) {
	if h.deps.DB == nil {
		return nil, fmt.Errorf("database is not ready")
	}

	axis := chart.Axis(period, time.Now().UTC())
	since, err := chartStart(axis)
	if err != nil {
		return nil, err
	}

	buckets, _, err := h.deps.DB.StatsSince(ctx, since, false)
	if err != nil {
		return nil, err
	}

	totals := make(map[string]int, len(buckets))
	for _, bucket := range buckets {
		totals[bucket.Bucket] = bucket.Total
	}

	points := chart.Points(axis, totals)
	payload := &statsPayload{
		Period: period,
		Mode:   mode,
		Points: points,
		Total:  chart.Total(points),
	}
	payload.Peak, payload.PeakAt = chart.Peak(points)
	payload.Avg = chart.Average(points)

	if mode == "split" {
		series, top, err := h.buildServiceSeries(ctx, axis, since, payload.Total)
		if err != nil {
			return nil, err
		}
		payload.Series = series
		payload.Top = top
	} else {
		payload.Series = []chart.Series{{Name: "Downloads", Points: points}}
	}

	users, err := h.deps.DB.CountUsers(ctx)
	if err != nil {
		return nil, err
	}
	active, err := h.deps.DB.CountUsersByStatus(ctx, "active")
	if err != nil {
		return nil, err
	}
	downloads, err := h.deps.DB.CountHistory(ctx, storage.HistoryFilter{})
	if err != nil {
		return nil, err
	}
	cached, err := h.deps.DB.CountHistory(ctx, storage.HistoryFilter{Status: storage.StatusCached})
	if err != nil {
		return nil, err
	}
	groups, err := h.deps.DB.TrackedGroupCount(ctx)
	if err != nil {
		return nil, err
	}

	payload.Users = users
	payload.Active = active
	payload.Groups = groups
	payload.All = downloads
	payload.Cached = cached

	return payload, nil
}

// buildServiceSeries builds one series per platform and the platform ranking.
func (h *Handler) buildServiceSeries(ctx context.Context, axis []chart.Bucket, since time.Time, total int) ([]chart.Series, []topService, error) {
	byService, err := h.deps.DB.StatsSinceByService(ctx, since)
	if err != nil {
		return nil, nil, err
	}

	type entry struct {
		meta   serviceMeta
		points []chart.Point
		count  int
	}

	entries := make([]entry, 0, len(byService))
	for service, counts := range byService {
		points := chart.Points(axis, counts)
		count := chart.Total(points)
		if count == 0 {
			continue
		}
		entries = append(entries, entry{meta: serviceMetaOf(service), points: points, count: count})
	}

	sort.SliceStable(entries, func(i, j int) bool {
		if entries[i].meta.Order != entries[j].meta.Order {
			return entries[i].meta.Order < entries[j].meta.Order
		}
		return entries[i].count > entries[j].count
	})

	series := make([]chart.Series, 0, len(entries))
	ranking := make([]topService, 0, len(entries))
	for _, item := range entries {
		series = append(series, chart.Series{Name: item.meta.Name, Points: item.points})
		share := 0.0
		if total > 0 {
			share = float64(item.count) * 100 / float64(total)
		}
		ranking = append(ranking, topService{
			Name:  item.meta.Name,
			Emoji: item.meta.Emoji,
			Count: item.count,
			Share: share,
		})
	}

	return series, ranking, nil
}

// chartStart returns the moment the first bucket of an axis starts.
func chartStart(axis []chart.Bucket) (time.Time, error) {
	if len(axis) == 0 {
		return time.Time{}, fmt.Errorf("the statistics axis is empty")
	}
	key := axis[0].Key
	layout := "2006-01-02"
	if len(key) == len("2006-01") {
		layout = "2006-01"
	}
	start, err := time.ParseInLocation(layout, key, time.UTC)
	if err != nil {
		return time.Time{}, fmt.Errorf("parse the statistics axis: %w", err)
	}
	return start, nil
}

// renderStatsChart draws the chart of the statistics screen.
func (h *Handler) renderStatsChart(payload *statsPayload) ([]byte, error) {
	modeLabel := "Overall activity"
	if payload.Mode == "split" {
		modeLabel = "Platform comparison"
	}

	bucketName := "day"
	if payload.Period == chart.PeriodYear {
		bucketName = "month"
	}

	options := chart.Options{
		Title:    "Downloads Overview",
		Subtitle: fmt.Sprintf("%s view | %s", payload.Period, modeLabel),
		Series:   payload.Series,
		Chips: []chart.Chip{
			{Title: "Total", Value: messages.FormatNumber(int64(payload.Total))},
			{Title: "Peak " + bucketName, Value: fmt.Sprintf("%s | %d", payload.PeakAt, payload.Peak)},
			{Title: "Average", Value: fmt.Sprintf("%.1f / %s", payload.Avg, bucketName)},
		},
	}

	data, err := chart.Render(options)
	if err != nil {
		return nil, err
	}
	return data, nil
}

// statsCaption renders the text under the statistics chart.
func statsCaption(payload *statsPayload) string {
	var builder strings.Builder
	builder.WriteString("<b>" + text.SymBar + " " + text.Bold(messages.BotName+" statistics") + "</b>\n\n")
	builder.WriteString(text.Label(text.SymRight, messages.BrandName+" "+payload.Period+" overview") + "\n\n")
	builder.WriteString(text.Label(text.SymUp, "users") + ": " + messages.FormatNumber(int64(payload.Users)) + "\n")
	builder.WriteString(text.Label(text.SymUpLeft, "groups") + ": " + messages.FormatNumber(int64(payload.Groups)) + "\n")
	builder.WriteString(text.Label(text.SymUp, "downloads") + ": " + messages.FormatNumber(int64(payload.All)) + "\n")
	builder.WriteString(text.Label(text.SymStar, "cached") + ": " + messages.FormatNumber(int64(payload.Cached)) + "\n")
	builder.WriteString(text.Label(text.SymCheck, "period events") + ": " + messages.FormatNumber(int64(payload.Total)) + "\n")

	if len(payload.Top) > 0 {
		builder.WriteString("\n<b>" + text.SymBar + " " + text.Bold("top platforms") + "</b>\n")
		for index, item := range payload.Top {
			if index >= 3 {
				break
			}
			builder.WriteString(fmt.Sprintf("%s %s: <b>%s</b> (%.0f%%)\n",
				item.Emoji, item.Name, messages.FormatNumber(int64(item.Count)), item.Share))
		}
	}
	return builder.String()
}
