package handlers

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"github.com/Iotatg/iota-downloader/internal/keyboards"
	"github.com/Iotatg/iota-downloader/internal/media"
	"github.com/Iotatg/iota-downloader/internal/messages"
	"github.com/Iotatg/iota-downloader/internal/platforms"
	"github.com/Iotatg/iota-downloader/internal/settings"
	"github.com/Iotatg/iota-downloader/internal/tg"
)

// inlineRequest is a pending inline delivery.
type inlineRequest struct {
	Service  string
	URL      string
	UserID   int64
	ChatID   int64
	Kind     string
	Settings settings.Set
	Created  time.Time
}

// inlineKey builds the state key of an inline request.
func inlineKey(prefix, token string) string { return prefix + ":" + token }

// newToken builds a random token for an inline request.
func newToken() string {
	buffer := make([]byte, 12)
	if _, err := rand.Read(buffer); err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(buffer)
}

// handleInlineQuery answers an inline query with the media preview.
func (h *Handler) handleInlineQuery(ctx context.Context, query *tg.InlineQuery) {
	if h.deps.Limiter != nil {
		if !h.deps.Limiter.Allow("inline:" + itoa(query.From.ID)) {
			_ = h.deps.TG.AnswerInlineQuery(ctx, query.ID, []tg.InlineQueryResult{}, 1, true, "")
			return
		}
	}

	service, rawURL, ok := platforms.ExtractLink(query.Query)
	if !ok {
		h.answerInlineHelp(ctx, query)
		return
	}

	resolved, err := h.deps.Services.Resolve(ctx, service, rawURL)
	if err != nil {
		h.logError("resolve inline link", err)
		_ = h.deps.TG.AnswerInlineQuery(ctx, query.ID, []tg.InlineQueryResult{}, 1, true, "")
		return
	}

	post := resolved.Post
	if post == nil || len(post.Media) == 0 {
		_ = h.deps.TG.AnswerInlineQuery(ctx, query.ID, []tg.InlineQueryResult{}, 1, true, "")
		return
	}

	set := h.userSettings(ctx, query.From.ID)
	token := newToken()
	request := &inlineRequest{
		Service:  resolved.Service,
		URL:      rawURL,
		UserID:   query.From.ID,
		Kind:     post.Media[0].Type,
		Settings: set,
		Created:  time.Now().UTC(),
	}
	if h.deps.State != nil {
		h.deps.State.Set(inlineKey("inline", token), request)
	}

	title := platforms.ServiceLabel(resolved.Service) + " " + post.Media[0].Type
	description := post.Description
	if description == "" {
		description = messages.InlineSendVideoPrompt(platforms.ServiceLabel(resolved.Service))
	}

	buttonLabel := messages.InlineSendVideoButton()
	switch post.Media[0].Type {
	case platforms.KindPhoto:
		buttonLabel = "Send photo inline"
	case platforms.KindAudio:
		buttonLabel = "Send audio inline"
	}

	thumb := post.Media[0].Thumb
	if thumb == "" && post.Metadata != nil {
		thumb = post.Metadata.Thumbnail
	}
	if thumb == "" {
		thumb = ""
	}

	result := tg.InlineQueryResult{
		Type:        "article",
		ID:          resolved.Service + "_inline:" + token,
		Title:       title,
		Description: description,
		ThumbURL:    thumb,
		InputMessageContent: &tg.InputTextMessageContent{
			MessageText:           messages.InlineSendVideoPrompt(platforms.ServiceLabel(resolved.Service)),
			ParseMode:             "HTML",
			DisableWebPagePreview: true,
		},
		ReplyMarkup: keyboards.InlineSendMediaKeyboard(buttonLabel, keyboards.PrefixInline+token),
	}

	if err := h.deps.TG.AnswerInlineQuery(ctx, query.ID, []tg.InlineQueryResult{result}, 10, true, ""); err != nil {
		h.logError("answer inline query", err)
	}
}

// answerInlineHelp answers an inline query without a link.
func (h *Handler) answerInlineHelp(ctx context.Context, query *tg.InlineQuery) {
	result := tg.InlineQueryResult{
		Type:        "article",
		ID:          "iota_help",
		Title:       messages.BrandName + " Downloader",
		Description: "Send a supported link to download it inline.",
		InputMessageContent: &tg.InputTextMessageContent{
			MessageText:           messages.Help(h.botUsername()),
			ParseMode:             "HTML",
			DisableWebPagePreview: true,
		},
	}
	_ = h.deps.TG.AnswerInlineQuery(ctx, query.ID, []tg.InlineQueryResult{result}, 5, true, "")
}

// handleInlineCallback delivers the media of an inline request.
func (h *Handler) handleInlineCallback(ctx context.Context, query *tg.CallbackQuery) {
	token := strings.TrimPrefix(query.Data, keyboards.PrefixInline)
	if token == "" || h.deps.State == nil {
		_ = h.deps.TG.AnswerCallbackQuery(ctx, query.ID, "Expired", true)
		return
	}

	value, ok := h.deps.State.Get(inlineKey("inline", token))
	if !ok {
		_ = h.deps.TG.AnswerCallbackQuery(ctx, query.ID, "Expired", true)
		return
	}
	request, ok := value.(*inlineRequest)
	if !ok {
		_ = h.deps.TG.AnswerCallbackQuery(ctx, query.ID, "Expired", true)
		return
	}

	chatID := query.Message.Chat.ID
	request.ChatID = chatID
	if h.deps.State != nil {
		h.deps.State.Set(inlineKey("inline", token), request)
	}

	_ = h.deps.TG.AnswerCallbackQuery(ctx, query.ID, "Preparing...", false)

	resolved, err := h.deps.Services.Resolve(ctx, request.Service, request.URL)
	if err != nil {
		h.logError("resolve inline request", err)
		h.answerText(ctx, chatID, messages.SomethingWentWrong())
		return
	}

	post := resolved.Post
	if post == nil || len(post.Media) == 0 {
		h.answerText(ctx, chatID, messages.NothingFound())
		return
	}

	item := post.Media[0]
	caption := h.captionForPost(resolved, post, request.Settings)
	markup := h.markupFor(resolved, post, request.Settings, 0, len(post.Media))

	status, err := h.deps.TG.SendMessage(ctx, chatID, messages.DownloadingVideoStatus(), tg.SendMessageOptions{
		ParseMode: "HTML",
	})
	if err != nil {
		h.logError("send inline status", err)
	}

	kind := item.Type
	if kind == "" {
		kind = platforms.KindVideo
	}
	opts := h.singleOptions(resolved, item, kind, caption, markup, request.Settings, status, chatID, query.From.ID, "private", 0, len(post.Media))
	if _, err := h.deps.Flow.RunSingle(ctx, opts); err != nil {
		h.logError("deliver inline media", err)
	}

	if status != nil {
		_ = h.deps.TG.DeleteMessage(ctx, chatID, status.MessageID)
	}
}

// handleAudioCallback downloads the audio of a delivered media item.
func (h *Handler) handleAudioCallback(ctx context.Context, query *tg.CallbackQuery) {
	data := strings.TrimPrefix(query.Data, keyboards.PrefixAudio)
	parts := strings.SplitN(data, ":", 2)
	if len(parts) != 2 {
		_ = h.deps.TG.AnswerCallbackQuery(ctx, query.ID, "Invalid request", true)
		return
	}

	service := parts[0]
	sourceURL := parts[1]
	chatID := query.Message.Chat.ID

	if service == platforms.ServiceYouTube {
		sourceURL = "https://www.youtube.com/watch?v=" + sourceURL
	}

	if h.deps.Dedupe != nil {
		leader, release := h.deps.Dedupe.Begin("audio:" + itoa(query.From.ID) + ":" + sourceURL)
		if !leader {
			_ = h.deps.TG.AnswerCallbackQuery(ctx, query.ID, "Already running", true)
			return
		}
		defer release()
	}

	_ = h.deps.TG.AnswerCallbackQuery(ctx, query.ID, "Preparing...", false)
	h.answerText(ctx, chatID, messages.DownloadingAudioStatus())

	set := h.userSettings(ctx, query.From.ID)
	if err := h.deliverAudioFor(ctx, chatID, query.From.ID, service, sourceURL, set); err != nil {
		h.logError("deliver audio button", err)
	}
}

// handleDocumentCallback sends the original file of a delivered media item.
func (h *Handler) handleDocumentCallback(ctx context.Context, query *tg.CallbackQuery) {
	data := strings.TrimPrefix(query.Data, keyboards.PrefixDocument)
	parts := strings.SplitN(data, ":", 2)
	if len(parts) != 2 {
		_ = h.deps.TG.AnswerCallbackQuery(ctx, query.ID, "Invalid request", true)
		return
	}

	service := parts[0]
	identifier := parts[1]
	chatID := query.Message.Chat.ID

	_ = h.deps.TG.AnswerCallbackQuery(ctx, query.ID, "Preparing...", false)

	serviceName := platforms.ServiceLabel(service)
	url := identifier
	if service == platforms.ServiceTikTok {
		url = "https://www.tiktok.com/@/video/" + identifier
	}

	h.answerText(ctx, chatID, messages.DownloadingVideoStatus())

	resolved, err := h.deps.Services.Resolve(ctx, service, url)
	if err != nil {
		h.logError("resolve document request", err)
		h.answerText(ctx, chatID, messages.SomethingWentWrong())
		return
	}
	if resolved.Post == nil || len(resolved.Post.Media) == 0 {
		h.answerText(ctx, chatID, messages.NothingFound())
		return
	}

	set := h.userSettings(ctx, query.From.ID)
	item := resolved.Post.Media[0]
	caption := h.captionForPost(resolved, resolved.Post, set)

	metrics, err := h.downloadMedia(ctx, resolved, item, set)
	if err != nil {
		h.logError("download document", err)
		h.answerText(ctx, chatID, messages.SomethingWentWrong())
		return
	}
	defer media.CleanupPath(metrics.Path)

	thumb := h.thumbnail(ctx, h.coverURL(resolved, item))
	defer media.CleanupThumbnail(thumb)

	if _, err := h.deps.TG.SendDocumentFile(ctx, chatID, metrics.Path, thumb, caption, "HTML", false, nil); err != nil {
		h.logError("send document", err)
		h.answerText(ctx, chatID, messages.SomethingWentWrong())
		return
	}
	_ = serviceName
}
