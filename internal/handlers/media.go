package handlers

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Iotatg/iota-downloader/internal/download"
	"github.com/Iotatg/iota-downloader/internal/keyboards"
	"github.com/Iotatg/iota-downloader/internal/media"
	"github.com/Iotatg/iota-downloader/internal/messages"
	"github.com/Iotatg/iota-downloader/internal/platforms"
	"github.com/Iotatg/iota-downloader/internal/services"
	"github.com/Iotatg/iota-downloader/internal/settings"
	"github.com/Iotatg/iota-downloader/internal/storage"
	"github.com/Iotatg/iota-downloader/internal/tg"
)

// handlePrivateLink processes the links of a private chat message.
func (h *Handler) handlePrivateLink(ctx context.Context, message *tg.Message, text string) {
	links := platforms.ExtractLinks(text)
	if len(links) == 0 {
		return
	}

	limit := h.batchLimit()
	detected := len(links)
	if len(links) > limit {
		links = links[:limit]
	}

	if len(links) > 1 {
		h.answerText(ctx, message.Chat.ID, messages.BatchLinksStarted(len(links), detected))
	}

	for index, link := range links {
		if index > 0 {
			h.answerText(ctx, message.Chat.ID, messages.BatchLinkProgress(index+1, len(links), platforms.ServiceLabel(link.Service)))
		}
		h.processLink(ctx, message, link)
	}

	if len(links) > 1 {
		h.answerText(ctx, message.Chat.ID, messages.BatchLinksFinished(len(links)))
	}
}

// handleGuestLink processes a link that was posted inside a group chat.
func (h *Handler) handleGuestLink(ctx context.Context, message *tg.Message, text string) {
	links := platforms.ExtractLinks(text)
	if len(links) == 0 {
		h.answerText(ctx, message.Chat.ID, messages.GuestUnsupportedLink())
		return
	}

	link := links[0]
	h.processLink(ctx, message, link)
}

// batchLimit returns the maximum number of links processed in one message.
func (h *Handler) batchLimit() int {
	if h.deps.Config == nil || h.deps.Config.BatchLinksMaxItems <= 0 {
		return 6
	}
	return h.deps.Config.BatchLinksMaxItems
}

// processLink resolves and delivers a single link.
func (h *Handler) processLink(ctx context.Context, message *tg.Message, link platforms.Link) {
	h.processLinkEntry(ctx, message, link, true)
}

// processLinkEntry delivers a link.
//
// The expand flag tells the handler whether the link may hold a collection,
// an entry of a collection never holds one, so a link of a playlist is
// delivered and never opens a menu of its own.
func (h *Handler) processLinkEntry(ctx context.Context, message *tg.Message, link platforms.Link, expand bool) {
	userID := message.From.ID
	chatID := message.Chat.ID
	chatType := message.Chat.Type

	if h.deps.Analytics != nil {
		h.deps.Analytics.Record("link_received", userID, chatType)
	}

	set := h.userSettings(ctx, userID)

	dedupeKey := fmt.Sprintf("link:%d:%s:%s", userID, link.Service, link.URL)
	if h.deps.Dedupe != nil {
		if h.deps.Dedupe.IsActive(dedupeKey) {
			h.answerText(ctx, chatID, messages.DuplicateLinkProcessing())
			return
		}
		if h.deps.Dedupe.WasRecentlyCompleted(dedupeKey) {
			h.answerText(ctx, chatID, messages.DuplicateLinkRecentlyProcessed())
			return
		}
	}

	// The answer comes first, the user must never wait in silence while the
	// platform is asked for the media.
	status := h.sendStatus(ctx, chatID, messages.ResolvingStatus(platforms.ServiceLabel(link.Service)))

	// A playlist, an album or a board holds many entries, the bot expands
	// it and offers the entries in a menu.
	if expand {
		if ref, ok := platforms.DetectCollection(link.Service, link.URL); ok {
			if h.showCollection(ctx, message, link, ref) {
				return
			}
			if status != nil {
				_ = h.deps.TG.DeleteMessage(ctx, chatID, status.MessageID)
			}
			status = nil
		}
	}

	started := time.Now()
	resolved, err := h.deps.Services.Resolve(ctx, link.Service, link.URL)
	h.logPerf("resolve", link.Service, started)
	if err != nil {
		h.logError("resolve link", err)
		h.replaceStatus(ctx, status, chatID, messages.LinkFailed(link.Service))
		return
	}

	post := resolved.Post
	if post == nil || len(post.Media) == 0 {
		h.replaceStatus(ctx, status, chatID, messages.NothingFound())
		return
	}

	if len(post.Media) == 1 && post.Media[0].Type == platforms.KindAudio {
		h.deliverAudio(ctx, message, resolved, set, status)
		return
	}

	if len(post.Media) > 1 {
		h.deliverAlbum(ctx, message, resolved, set, status)
		return
	}

	h.deliverSingle(ctx, message, resolved, set, status, 0, len(post.Media))
}

// sendStatus sends a status message into a chat.
func (h *Handler) sendStatus(ctx context.Context, chatID int64, body string) *tg.Message {
	status, err := h.deps.TG.SendMessage(ctx, chatID, body, tg.SendMessageOptions{ParseMode: "HTML"})
	if err != nil {
		h.logError("send status", err)
		return nil
	}
	return status
}

// updateStatus rewrites an already sent status message.
func (h *Handler) updateStatus(ctx context.Context, status *tg.Message, chatID int64, body string) {
	if status == nil {
		return
	}
	if _, err := h.deps.TG.EditMessageText(ctx, chatID, status.MessageID, body, "HTML", nil); err != nil {
		h.logError("edit status", err)
	}
}

// replaceStatus rewrites a status message, it sends a new message when the
// status message is gone already.
func (h *Handler) replaceStatus(ctx context.Context, status *tg.Message, chatID int64, body string) {
	if status == nil {
		h.answerText(ctx, chatID, body)
		return
	}
	if _, err := h.deps.TG.EditMessageText(ctx, chatID, status.MessageID, body, "HTML", nil); err != nil {
		h.logError("edit status", err)
		h.answerText(ctx, chatID, body)
	}
}

// answerText sends a short answer into a chat.
func (h *Handler) answerText(ctx context.Context, chatID int64, text string) error {
	if err := h.reply(ctx, chatID, text, nil); err != nil {
		h.logError("send answer", err)
		return err
	}
	return nil
}

// deliverSingle downloads and sends a single media item.
func (h *Handler) deliverSingle(ctx context.Context, message *tg.Message, resolved *services.Resolved, set settings.Set, status *tg.Message, index, total int) {
	post := resolved.Post
	item := post.Media[index]
	chatID := message.Chat.ID
	userID := message.From.ID
	chatType := message.Chat.Type

	h.updateStatus(ctx, status, chatID, messages.DownloadingVideoStatus())

	kind := item.Type
	if kind == "" {
		kind = platforms.KindVideo
	}

	caption := h.captionForPost(resolved, post, set)
	markup := h.markupFor(resolved, post, set, index, total)

	opts := h.singleOptions(resolved, item, kind, caption, markup, set, status, chatID, userID, chatType, index, total)

	if _, err := h.deps.Flow.RunSingle(ctx, opts); err != nil {
		h.logError("deliver media", err)
	}

	if status != nil {
		_ = h.deps.TG.DeleteMessage(ctx, chatID, status.MessageID)
	}
	h.cleanupTrigger(ctx, message, set)
}

// deliverAlbum downloads and sends an album of media items.
func (h *Handler) deliverAlbum(ctx context.Context, message *tg.Message, resolved *services.Resolved, set settings.Set, status *tg.Message) {
	post := resolved.Post
	chatID := message.Chat.ID
	userID := message.From.ID
	chatType := message.Chat.Type
	total := len(post.Media)

	h.updateStatus(ctx, status, chatID, messages.DownloadingVideoStatus())

	items := make([]media.GroupItem, 0, total)
	for index, item := range post.Media {
		indexCopy := index
		items = append(items, media.GroupItem{
			Index:    index,
			Kind:     item.Type,
			CacheKey: h.cacheKey(resolved, item, index, total, set),
			URL:      item.URL,
			Download: func(ctx context.Context) (*download.Metrics, error) {
				return h.downloadMedia(ctx, resolved, post.Media[indexCopy], set)
			},
		})
	}

	opts := media.GroupOptions{
		Items:  items,
		Source: resolved.Service,
		UpdateStatus: func(ctx context.Context, text string) error {
			if status == nil {
				return nil
			}
			_, err := h.deps.TG.EditMessageText(ctx, chatID, status.MessageID, text, "HTML", nil)
			return err
		},
		SendEntries: func(ctx context.Context, entries []media.GroupEntry) error {
			return h.sendAlbum(ctx, chatID, entries, post, set, resolved)
		},
		OnEmpty: func(ctx context.Context) error {
			return h.answerText(ctx, chatID, messages.NothingFound())
		},
		Cleanup: media.CleanupPath,
		Record: func(ctx context.Context, status string, err error) {
			h.recordDownload(ctx, userID, chatID, chatType, resolved, post, post.Media[0], "", status, err)
		},
	}

	if err := h.deps.Flow.RunGroup(ctx, opts); err != nil {
		h.logError("deliver album", err)
	}

	if status != nil {
		_ = h.deps.TG.DeleteMessage(ctx, chatID, status.MessageID)
	}
	h.cleanupTrigger(ctx, message, set)
}

// deliverAudio downloads and sends an audio track.
func (h *Handler) deliverAudio(ctx context.Context, message *tg.Message, resolved *services.Resolved, set settings.Set, status *tg.Message) {
	post := resolved.Post
	chatID := message.Chat.ID
	userID := message.From.ID
	chatType := message.Chat.Type
	item := post.Media[0]

	sourceURL := ""
	if post.Metadata != nil {
		sourceURL = post.Metadata.SourceURL
	}
	if sourceURL == "" {
		sourceURL = item.URL
	}

	h.updateStatus(ctx, status, chatID, messages.DownloadingAudioStatus())

	title := audioTitle(post)
	performer := audioPerformer(post)

	var thumb string
	opts := media.AudioOptions{
		CacheKey:      media.AudioCacheKey(sourceURL, set.AudioFormat),
		CacheFileType: "audio",
		Source:        resolved.Service,
		Title:         title,
		Performer:     performer,
		DownloadAudio: func(ctx context.Context) (*download.Metrics, error) {
			return h.downloadAudio(ctx, resolved, item, set)
		},
		Prepare: func(ctx context.Context, path string) (string, error) {
			prepared, cover, err := h.prepareAudio(ctx, resolved, path, set)
			thumb = cover
			return prepared, err
		},
		Send: func(ctx context.Context, path string, duration int) (*tg.Message, error) {
			return h.sendAudio(ctx, chatID, path, duration, title, performer, thumb)
		},
		SendCached: func(ctx context.Context, fileID string) (*tg.Message, error) {
			return h.sendCachedAudio(ctx, chatID, fileID, title, performer)
		},
		Cleanup: media.CleanupPath,
		OnMissingAudio: func(ctx context.Context) error {
			return h.answerText(ctx, chatID, messages.AudioFetchFailed())
		},
		OnTooLarge: func(ctx context.Context) error {
			return h.answerText(ctx, chatID, messages.AudioTooLarge())
		},
		Record: func(ctx context.Context, status string, err error) {
			h.recordDownload(ctx, userID, chatID, chatType, resolved, post, item, "", status, err)
		},
	}

	if _, err := h.deps.Flow.RunAudio(ctx, opts); err != nil {
		h.logError("deliver audio", err)
	}

	if status != nil {
		_ = h.deps.TG.DeleteMessage(ctx, chatID, status.MessageID)
	}
	h.cleanupTrigger(ctx, message, set)
}

// cacheKey builds the cache key of a media item.
func (h *Handler) cacheKey(resolved *services.Resolved, item platforms.Media, index, total int, set settings.Set) string {
	kind := item.Type
	if kind == "" {
		kind = platforms.KindVideo
	}

	switch resolved.Service {
	case platforms.ServiceYouTube:
		base := ""
		if resolved.Post != nil && resolved.Post.Metadata != nil {
			base = resolved.Post.Metadata.SourceURL
		}
		if base == "" {
			base = item.URL
		}
		return media.YouTubeVideoCacheKey(base, set.VideoQuality)

	case platforms.ServiceTikTok:
		id := ""
		if resolved.Post != nil {
			id = resolved.Post.ID
		}
		return media.TikTokVideoCacheKey(id)

	case platforms.ServiceTwitter:
		url := ""
		if resolved.Post != nil && resolved.Post.Metadata != nil {
			url = resolved.Post.Metadata.SourceURL
		}
		return media.TwitterMediaCacheKey(url, index, kind, total)

	default:
		base := ""
		if resolved.Post != nil && resolved.Post.Metadata != nil {
			base = resolved.Post.Metadata.SourceURL
		}
		if base == "" {
			base = item.URL
		}
		return fmt.Sprintf("%s#item:%d:%s", base, index, kind)
	}
}

// captionForPost builds the caption of a media item including its platform.
func (h *Handler) captionForPost(resolved *services.Resolved, post *platforms.Post, set settings.Set) string {
	description := ""
	author := ""
	if post != nil {
		description = post.Description
		author = post.Author
	}
	service := ""
	if resolved != nil {
		service = platforms.ServiceLabel(resolved.Service)
	}
	return messages.MediaCaption(set.Captions, service, author, description, h.botLink(), 1024)
}

// markupFor builds the inline keyboard of a media item.
func (h *Handler) markupFor(resolved *services.Resolved, post *platforms.Post, set settings.Set, index, total int) *tg.InlineKeyboardMarkup {
	if post == nil {
		return nil
	}

	info := keyboards.MediaInfo{VideoURL: post.Metadata.SourceURL}
	if post.Info != nil {
		views := post.Info.Views
		likes := post.Info.Likes
		comments := post.Info.Comments
		shares := post.Info.Shares
		info.Views = &views
		info.Likes = &likes
		info.Comments = &comments
		info.Shares = &shares
	}

	audioCallback := ""
	fileCallback := ""
	sourceURL := ""
	if post.Metadata != nil {
		sourceURL = post.Metadata.SourceURL
	}
	switch resolved.Service {
	case platforms.ServiceYouTube:
		audioCallback = "audio:youtube:" + post.ID
	case platforms.ServiceTikTok:
		audioCallback = "audio:tiktok:" + post.ID
		fileCallback = "doc:tiktok:" + post.ID
	case platforms.ServiceInstagram:
		audioCallback = "audio:inst:" + sourceURL
	}

	return keyboards.VideoInfoKeyboard(info, set, audioCallback, fileCallback)
}

// mtprotoFileLimit is the size limit of a single upload over MTProto, the
// server refuses every file part at or beyond 2000 MB.
const mtprotoFileLimit = int64(2000) * 1024 * 1024

// sendMedia sends a downloaded media file.
//
// The optional cover is uploaded as the thumbnail of the message.
//
// A file that is bigger than the limit of a single upload is split into parts,
// the server refuses every part at or beyond 2000 MB.
func (h *Handler) sendMedia(ctx context.Context, chatID int64, path, kind, caption string, markup *tg.InlineKeyboardMarkup, set settings.Set, cover string) (*tg.Message, error) {
	started := time.Now()
	defer func() { h.logPerf("send_media", kind, started) }()

	if h.exceedsMTProtoLimit(path) {
		return h.sendSplitMedia(ctx, chatID, path, kind, caption, markup)
	}

	thumb := ""
	if kind != platforms.KindPhoto {
		thumb = h.thumbnail(ctx, cover)
		defer media.CleanupThumbnail(thumb)
	}

	switch kind {
	case platforms.KindPhoto:
		if set.IsOn(settings.FieldAsDocument) {
			return h.deps.TG.SendDocumentFile(ctx, chatID, path, thumb, caption, "HTML", false, markup)
		}
		return h.deps.TG.SendPhotoFile(ctx, chatID, path, caption, "HTML", markup)
	case platforms.KindAudio:
		return h.deps.TG.SendAudioFile(ctx, chatID, path, thumb, caption, "HTML", 0, "", "", markup)
	default:
		return h.deps.TG.SendVideoFile(ctx, chatID, path, thumb, caption, "HTML", 0, 0, 0, true, markup)
	}
}

// exceedsMTProtoLimit reports whether a file is too big for a single upload
// over MTProto.
func (h *Handler) exceedsMTProtoLimit(path string) bool {
	limit := h.mtprotoLimit()
	if limit <= 0 {
		return false
	}
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return info.Size() > limit
}

// mtprotoLimit returns the size limit of a single upload over MTProto.
func (h *Handler) mtprotoLimit() int64 {
	if h.deps.Config == nil || h.deps.Config.MTProtoFileLimitMB <= 0 {
		return mtprotoFileLimit
	}
	return int64(h.deps.Config.MTProtoFileLimitMB) * 1024 * 1024
}

// sendSplitMedia delivers a file that is too big as a series of parts.
func (h *Handler) sendSplitMedia(ctx context.Context, chatID int64, path, kind, caption string, markup *tg.InlineKeyboardMarkup) (*tg.Message, error) {
	split, err := media.SplitFile(path, h.partSize())
	if err != nil {
		return nil, err
	}
	defer media.RemoveParts(split)

	for _, part := range split.Parts {
		partCaption := caption
		if part.Index == 1 {
			partCaption = messages.SplitPartCaption(caption, len(split.Parts), split.Original)
		}
		if err := h.sendPart(ctx, chatID, part, partCaption); err != nil {
			return nil, err
		}
	}

	return &tg.Message{MessageID: 0, Chat: &tg.Chat{ID: chatID}}, nil
}

// sendPart delivers a single part of a split file.
func (h *Handler) sendPart(ctx context.Context, chatID int64, part media.Part, caption string) error {
	_, err := h.deps.TG.SendDocumentFile(ctx, chatID, part.Path, "", caption, "HTML", false, nil)
	return err
}

// partSize returns the size of a single part of a split file.
//
// The parts stay below the limit of a single upload, a small margin keeps the
// upload away from the limit.
func (h *Handler) partSize() int64 {
	return partSizeOf(h.mtprotoLimit())
}

// partSizeOf keeps a small margin below the given limit.
func partSizeOf(limit int64) int64 {
	if limit <= 0 {
		return media.DefaultPartSize
	}
	partSize := limit * 95 / 100
	if partSize <= 0 {
		return media.DefaultPartSize
	}
	return partSize
}

// sendCachedMedia sends an already uploaded media file.
//
// The cache holds the MTProto reference of the file, the file is copied to
// the chat without a new upload.
func (h *Handler) sendCachedMedia(ctx context.Context, chatID int64, fileID, kind, caption string, markup *tg.InlineKeyboardMarkup) (*tg.Message, error) {
	return h.deps.TG.SendCachedFile(ctx, chatID, fileID, caption, markup)
}

// sendAlbum sends the entries of an album one by one.
func (h *Handler) sendAlbum(ctx context.Context, chatID int64, entries []media.GroupEntry, post *platforms.Post, set settings.Set, resolved *services.Resolved) error {
	started := time.Now()
	defer func() { h.logPerf("send_album", "", started) }()

	caption := h.captionForPost(resolved, post, set)
	markup := h.markupFor(resolved, post, set, 0, len(entries))

	for _, entry := range entries {
		kind := entry.Kind
		if kind == "" {
			kind = platforms.KindVideo
		}
		itemCaption := caption
		if entry.Index > 0 {
			itemCaption = ""
		}

		cover := ""
		if entry.Index < len(post.Media) {
			cover = h.coverURL(resolved, post.Media[entry.Index])
		}

		var err error
		if entry.Cached {
			_, err = h.sendCachedMedia(ctx, chatID, entry.FileID, kind, itemCaption, markup)
		} else {
			_, err = h.sendMedia(ctx, chatID, entry.Path, kind, itemCaption, markup, set, cover)
		}
		if err != nil {
			return err
		}
	}
	return nil
}

// sendAudio sends a downloaded audio file.
func (h *Handler) sendAudio(ctx context.Context, chatID int64, path string, duration int, title, performer, thumb string) (*tg.Message, error) {
	started := time.Now()
	defer func() { h.logPerf("send_audio", "", started) }()

	if h.exceedsMTProtoLimit(path) {
		return h.sendSplitMedia(ctx, chatID, path, platforms.KindAudio, "", nil)
	}

	return h.deps.TG.SendAudioFile(ctx, chatID, path, thumb, "", "HTML", duration, performer, title, nil)
}

// sendCachedAudio sends an already uploaded audio file.
func (h *Handler) sendCachedAudio(ctx context.Context, chatID int64, fileID, title, performer string) (*tg.Message, error) {
	return h.deps.TG.SendCachedFile(ctx, chatID, fileID, "", nil)
}

// cleanupTrigger removes the trigger message when the user asked for it.
func (h *Handler) cleanupTrigger(ctx context.Context, message *tg.Message, set settings.Set) {
	if !set.IsOn(settings.FieldDeleteMessage) {
		return
	}
	if !h.deleteMessage(ctx, message.Chat.ID, message.MessageID) {
		h.answerText(ctx, message.Chat.ID, messages.DeletePermissionWarning())
	}
}

// recordDownload writes an entry into the download history.
func (h *Handler) recordDownload(ctx context.Context, userID, chatID int64, chatType string, resolved *services.Resolved, post *platforms.Post, item platforms.Media, fileType string, status string, err error) {
	if h.deps.DB == nil {
		return
	}

	record := &storage.DownloadRecord{
		UserID:   userID,
		ChatID:   &chatID,
		ChatType: chatType,
		Service:  resolved.Service,
		Status:   status,
	}
	if post != nil {
		if post.Metadata != nil {
			record.URL = post.Metadata.SourceURL
			record.Title = post.Metadata.Title
		}
		if record.Title == "" {
			record.Title = post.Description
		}
	}
	if record.URL == "" {
		record.URL = item.URL
	}
	record.FileType = fileType
	if record.FileType == "" {
		record.FileType = item.Type
	}
	if err != nil {
		record.ErrorMessage = err.Error()
	}

	if writeErr := h.deps.DB.RecordDownload(ctx, record); writeErr != nil {
		h.logError("record download", writeErr)
	}
}

// audioTitle returns the title of an audio item.
func audioTitle(post *platforms.Post) string {
	if post == nil || post.Metadata == nil {
		return ""
	}
	return post.Metadata.Title
}

// audioPerformer returns the performer of an audio item.
func audioPerformer(post *platforms.Post) string {
	if post == nil || post.Metadata == nil {
		return ""
	}
	return post.Metadata.Artists
}

// coverURL returns the cover image of a media item.
func (h *Handler) coverURL(resolved *services.Resolved, item platforms.Media) string {
	if item.Thumb != "" {
		return item.Thumb
	}
	if resolved == nil || resolved.Post == nil {
		return ""
	}
	if resolved.Post.Info != nil && resolved.Post.Info.Cover != "" {
		return resolved.Post.Info.Cover
	}
	if resolved.Post.Metadata != nil && resolved.Post.Metadata.Thumbnail != "" {
		return resolved.Post.Metadata.Thumbnail
	}
	return ""
}

// thumbnail prepares the cover of an item for Telegram.
//
// An empty path is returned when there is no cover or when the download
// fails, the media is then sent without a thumbnail.
func (h *Handler) thumbnail(ctx context.Context, cover string) string {
	if cover == "" {
		return ""
	}
	directory, err := os.MkdirTemp(h.coverDir(), "cover-")
	if err != nil {
		h.logError("create cover directory", err)
		return ""
	}
	prepared, err := media.Thumbnail(ctx, cover, directory)
	if err != nil {
		h.logError("prepare thumbnail", err)
		media.CleanupThumbnail(filepath.Join(directory, "cover.jpg"))
		return ""
	}
	if prepared == "" {
		media.CleanupThumbnail(filepath.Join(directory, "cover.jpg"))
		return ""
	}
	return prepared
}

// coverDir returns the directory that holds the cover images.
func (h *Handler) coverDir() string {
	base := ""
	if h.deps.Config != nil {
		base = h.deps.Config.OutputDir
	}
	if base == "" {
		base = os.TempDir()
	}
	directory := filepath.Join(base, "covers")
	if err := os.MkdirAll(directory, 0o755); err != nil {
		return base
	}
	return directory
}

// prepareAudio converts an audio file, writes its tags and embeds the cover.
//
// It returns the final path and the thumbnail of the cover.
func (h *Handler) prepareAudio(ctx context.Context, resolved *services.Resolved, path string, set settings.Set) (string, string, error) {
	final := path

	converted, err := media.ConvertAudioFormat(ctx, final, settings.AudioCodec(set.AudioFormat), settings.AudioBitrate(set.AudioFormat), h.riteshBackend(resolved))
	if err != nil {
		h.logError("convert audio", err)
	}
	if converted != "" {
		final = converted
	}

	tagged := media.PrepareAudioFile(ctx, final, h.audioMetadata(resolved))
	if tagged != "" {
		final = tagged
	}

	cover := ""
	if resolved != nil && resolved.Post != nil && len(resolved.Post.Media) > 0 {
		cover = h.coverURL(resolved, resolved.Post.Media[0])
	}
	if cover == "" {
		return final, "", nil
	}

	thumbnail := h.thumbnail(ctx, cover)
	if thumbnail == "" {
		return final, "", nil
	}
	if embedded, ok := media.EmbedAudioCover(ctx, final, thumbnail); ok {
		final = embedded
	}
	return final, thumbnail, nil
}

// riteshBackend reports whether the media comes from the Ritesh api.
//
// The api serves the file with a bitrate of its own, the file is always
// converted into the format of the user.
func (h *Handler) riteshBackend(resolved *services.Resolved) bool {
	return resolved != nil && resolved.Backend == platforms.BackendRitesh
}

// usesYTDLP reports whether the media is downloaded with yt-dlp.
//
// YouTube and the platforms that serve their media as a protected stream are
// downloaded with yt-dlp, the Ritesh api delivers a ready made file.
func (h *Handler) usesYTDLP(resolved *services.Resolved) bool {
	if resolved == nil || resolved.Backend == platforms.BackendRitesh {
		return false
	}
	return resolved.Backend == platforms.BackendYTDLP || resolved.Service == platforms.ServiceYouTube
}

// ytdlpURL returns the address that yt-dlp downloads.
//
// The resolver of YouTube hands over the direct link of a single stream, the
// quality of the video is chosen by the format selector and needs the page of
// the video. The direct link of a stream is kept for the links that are
// fetched as a plain file.
func ytdlpURL(resolved *services.Resolved, item platforms.Media) string {
	if resolved != nil && resolved.Post != nil && resolved.Post.Metadata != nil {
		if source := resolved.Post.Metadata.SourceURL; source != "" {
			return source
		}
	}
	return item.URL
}

// audioMetadata builds the tags of an audio file.
func (h *Handler) audioMetadata(resolved *services.Resolved) media.Metadata {
	metadata := media.Metadata{}
	if resolved == nil || resolved.Post == nil {
		return metadata
	}
	if resolved.Post.Metadata != nil {
		metadata.Title = resolved.Post.Metadata.Title
		metadata.Artists = resolved.Post.Metadata.Artists
		metadata.Album = resolved.Post.Metadata.Album
		metadata.AlbumArtist = resolved.Post.Metadata.AlbumArtist
		metadata.Date = resolved.Post.Metadata.Date
		metadata.TrackNumber = resolved.Post.Metadata.TrackNumber
		metadata.DiscNumber = resolved.Post.Metadata.DiscNumber
		metadata.SourceURL = resolved.Post.Metadata.SourceURL
	}
	if metadata.Title == "" {
		metadata.Title = resolved.Post.Description
	}
	return metadata
}

// downloadMedia downloads a single media item.
func (h *Handler) downloadMedia(ctx context.Context, resolved *services.Resolved, item platforms.Media, set settings.Set) (*download.Metrics, error) {
	started := time.Now()

	if h.usesYTDLP(resolved) {
		filename := h.filenameFor(resolved, item, ".mp4")
		selector := settings.VideoFormatSelector(set.VideoQuality)
		path, err := h.deps.Services.YouTube.DownloadVideo(ctx, ytdlpURL(resolved, item), filename, selector, h.maxFileSize())
		if err != nil {
			return nil, err
		}
		metrics, metricsErr := h.metricsFor(path)
		h.logPerf("download_video", resolved.Service, started)
		return metrics, metricsErr
	}

	extension := ""
	if resolved.Backend == platforms.BackendRitesh {
		// The api delivers a ready made file, the name needs the extension
		// of the container.
		if item.Type == platforms.KindVideo {
			extension = ".mp4"
		} else {
			extension = ".mp3"
		}
	}

	metrics, err := h.deps.Downloader.Download(ctx, item.URL, h.filenameFor(resolved, item, extension), download.Options{
		MaxSize: h.maxFileSize(),
		Headers: resolved.Headers,
	})
	h.logPerf("download_media", resolved.Service, started)
	return metrics, err
}

// downloadAudio downloads an audio track.
func (h *Handler) downloadAudio(ctx context.Context, resolved *services.Resolved, item platforms.Media, set settings.Set) (*download.Metrics, error) {
	codec := settings.AudioCodec(set.AudioFormat)
	bitrate := settings.AudioBitrate(set.AudioFormat)
	started := time.Now()

	if h.usesYTDLP(resolved) {
		path, err := h.deps.Services.YouTube.DownloadAudio(ctx, ytdlpURL(resolved, item), h.filenameFor(resolved, item, ""), h.maxFileSize(), codec, bitrate)
		if err != nil {
			return nil, err
		}
		metrics, metricsErr := h.metricsFor(path)
		h.logPerf("download_audio", resolved.Service, started)
		return metrics, metricsErr
	}

	extension := "." + codec
	if resolved.Backend == platforms.BackendRitesh {
		// The api always serves an mp3, the name must carry the real
		// container so that the conversion into the wanted format works.
		extension = ".mp3"
	}

	metrics, err := h.deps.Downloader.Download(ctx, item.URL, h.filenameFor(resolved, item, extension), download.Options{
		MaxSize: h.maxFileSize(),
		Headers: resolved.Headers,
	})
	h.logPerf("download_audio", resolved.Service, started)
	return metrics, err
}

// metricsFor builds the download metrics of a local file.
func (h *Handler) metricsFor(path string) (*download.Metrics, error) {
	if path == "" {
		return nil, errors.New("no file was produced")
	}
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	return &download.Metrics{Path: path, Size: info.Size()}, nil
}

// maxFileSize returns the maximum size of a delivered file.
func (h *Handler) maxFileSize() int64 {
	if h.deps.Config == nil {
		return 0
	}
	return h.deps.Config.MaxFileSize
}

// filenameFor builds the file name of a download.
func (h *Handler) filenameFor(resolved *services.Resolved, item platforms.Media, extension string) string {
	title := ""
	if resolved.Post != nil && resolved.Post.Metadata != nil {
		title = resolved.Post.Metadata.Title
	}
	if title == "" && resolved.Post != nil {
		title = resolved.Post.Description
	}
	if title == "" {
		title = "download"
	}
	name := platforms.SanitizeFilename(title, 80)
	if extension == "" {
		return name
	}
	if strings.HasSuffix(name, extension) {
		return name
	}
	return name + extension
}

// trimText shortens a text to the given amount of runes.
func trimText(value string, limit int) string {
	runes := []rune(value)
	if len(runes) <= limit {
		return value
	}
	return string(runes[:limit])
}

// singleOptions builds the delivery options of a single media item.
func (h *Handler) singleOptions(resolved *services.Resolved, item platforms.Media, kind, caption string, markup *tg.InlineKeyboardMarkup, set settings.Set, status *tg.Message, chatID, userID int64, chatType string, index, total int) media.SingleOptions {
	return media.SingleOptions{
		CacheKey:      h.cacheKey(resolved, item, index, total, set),
		CacheFileType: kind,
		Source:        resolved.Service,
		Kind:          kind,
		UpdateStatus: func(ctx context.Context, text string) error {
			if status == nil {
				return nil
			}
			_, err := h.deps.TG.EditMessageText(ctx, chatID, status.MessageID, text, "HTML", nil)
			return err
		},
		SendChatAction: func(ctx context.Context, action string) error {
			return h.deps.TG.SendChatAction(ctx, chatID, action)
		},
		Download: func(ctx context.Context) (*download.Metrics, error) {
			return h.downloadMedia(ctx, resolved, item, set)
		},
		Send: func(ctx context.Context, path string) (*tg.Message, error) {
			return h.sendMedia(ctx, chatID, path, kind, caption, markup, set, h.coverURL(resolved, item))
		},
		SendCached: func(ctx context.Context, fileID string) (*tg.Message, error) {
			return h.sendCachedMedia(ctx, chatID, fileID, kind, caption, markup)
		},
		Cleanup: media.CleanupPath,
		OnMissingMedia: func(ctx context.Context) error {
			return h.answerText(ctx, chatID, messages.NothingFound())
		},
		OnRateLimit: func(ctx context.Context, wait time.Duration) error {
			seconds := int(wait.Seconds()) + 1
			return h.answerText(ctx, chatID, messages.RateLimitText(seconds))
		},
		OnQueueBusy: func(ctx context.Context, position int) error {
			return h.answerText(ctx, chatID, messages.QueueBusyText(position))
		},
		OnTooLarge: func(ctx context.Context) error {
			return h.answerText(ctx, chatID, messages.VideoTooLarge())
		},
		OnUnexpectedError: func(ctx context.Context, err error) error {
			return h.answerText(ctx, chatID, messages.SomethingWentWrong())
		},
		Record: func(ctx context.Context, status string, err error) {
			h.recordDownload(ctx, userID, chatID, chatType, resolved, resolved.Post, item, kind, status, err)
		},
	}
}

// deliverAudioFor downloads and delivers an audio track of a known source.
func (h *Handler) deliverAudioFor(ctx context.Context, chatID, userID int64, service, sourceURL string, set settings.Set) error {
	resolved, err := h.deps.Services.ResolveKind(ctx, service, sourceURL, platforms.KindAudio)
	if err != nil {
		h.logError("resolve audio request", err)
		return h.answerText(ctx, chatID, messages.AudioFetchFailed())
	}
	if resolved.Post == nil || len(resolved.Post.Media) == 0 {
		return h.answerText(ctx, chatID, messages.AudioFetchFailed())
	}

	post := resolved.Post
	item := post.Media[0]
	title := audioTitle(post)
	performer := audioPerformer(post)

	var thumb string
	opts := media.AudioOptions{
		CacheKey:      media.AudioCacheKey(sourceURL, set.AudioFormat),
		CacheFileType: "audio",
		Source:        service,
		Title:         title,
		Performer:     performer,
		DownloadAudio: func(ctx context.Context) (*download.Metrics, error) {
			return h.downloadAudio(ctx, resolved, item, set)
		},
		Prepare: func(ctx context.Context, path string) (string, error) {
			prepared, cover, err := h.prepareAudio(ctx, resolved, path, set)
			thumb = cover
			return prepared, err
		},
		Send: func(ctx context.Context, path string, duration int) (*tg.Message, error) {
			return h.sendAudio(ctx, chatID, path, duration, title, performer, thumb)
		},
		SendCached: func(ctx context.Context, fileID string) (*tg.Message, error) {
			return h.sendCachedAudio(ctx, chatID, fileID, title, performer)
		},
		Cleanup: media.CleanupPath,
		OnMissingAudio: func(ctx context.Context) error {
			return h.answerText(ctx, chatID, messages.AudioFetchFailed())
		},
		OnTooLarge: func(ctx context.Context) error {
			return h.answerText(ctx, chatID, messages.AudioTooLarge())
		},
		Record: func(ctx context.Context, status string, err error) {
			h.recordDownload(ctx, userID, chatID, "private", resolved, post, item, platforms.KindAudio, status, err)
		},
	}

	if _, err := h.deps.Flow.RunAudio(ctx, opts); err != nil {
		return err
	}
	return nil
}
