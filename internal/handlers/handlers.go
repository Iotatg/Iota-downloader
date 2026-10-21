// Package handlers contains the update processing of the Iota Coder bot.
package handlers

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/Iotatg/iota-downloader/config"
	"github.com/Iotatg/iota-downloader/internal/download"
	"github.com/Iotatg/iota-downloader/internal/logger"
	"github.com/Iotatg/iota-downloader/internal/media"
	"github.com/Iotatg/iota-downloader/internal/runtime"
	"github.com/Iotatg/iota-downloader/internal/services"
	"github.com/Iotatg/iota-downloader/internal/settings"
	"github.com/Iotatg/iota-downloader/internal/storage"
	"github.com/Iotatg/iota-downloader/internal/tg"
)

// Telegram is the part of the Telegram client that the handlers use.
//
// The client speaks MTProto, the interface keeps the handlers testable
// without a connection to Telegram.
type Telegram interface {
	GetMe(ctx context.Context) (*tg.User, error)
	SendMessage(ctx context.Context, chatID int64, text string, opts tg.SendMessageOptions) (*tg.Message, error)
	EditMessageText(ctx context.Context, chatID int64, messageID int64, text string, parseMode string, markup *tg.InlineKeyboardMarkup) (*tg.Message, error)
	EditMessageCaption(ctx context.Context, chatID int64, messageID int64, caption string, parseMode string, markup *tg.InlineKeyboardMarkup) (*tg.Message, error)
	SendChatAction(ctx context.Context, chatID int64, action string) error
	AnswerCallbackQuery(ctx context.Context, callbackID string, text string, showAlert bool) error
	AnswerInlineQuery(ctx context.Context, inlineQueryID string, results []tg.InlineQueryResult, cacheTime int, isPersonal bool, nextOffset string) error
	SendPhoto(ctx context.Context, chatID int64, media string, caption string, parseMode string, markup *tg.InlineKeyboardMarkup, hasSpoiler bool) (*tg.Message, error)
	SendPhotoFile(ctx context.Context, chatID int64, path string, caption string, parseMode string, markup *tg.InlineKeyboardMarkup) (*tg.Message, error)
	SendPhotoBytes(ctx context.Context, chatID int64, data []byte, filename, caption, parseMode string, markup *tg.InlineKeyboardMarkup) (*tg.Message, error)
	EditMessageMediaPhoto(ctx context.Context, chatID int64, messageID int64, data []byte, filename, caption, parseMode string, markup *tg.InlineKeyboardMarkup) (*tg.Message, error)
	SendVideo(ctx context.Context, chatID int64, media string, caption string, parseMode string, duration int, width, height int, supportsStreaming bool, markup *tg.InlineKeyboardMarkup) (*tg.Message, error)
	SendVideoFile(ctx context.Context, chatID int64, path string, thumbPath string, caption string, parseMode string, duration int, width, height int, supportsStreaming bool, markup *tg.InlineKeyboardMarkup) (*tg.Message, error)
	SendDocument(ctx context.Context, chatID int64, media string, caption string, parseMode string, disableContentTypeDetection bool, markup *tg.InlineKeyboardMarkup) (*tg.Message, error)
	SendDocumentFile(ctx context.Context, chatID int64, path string, thumbPath string, caption string, parseMode string, disableContentTypeDetection bool, markup *tg.InlineKeyboardMarkup) (*tg.Message, error)
	SendAudio(ctx context.Context, chatID int64, media string, caption string, parseMode string, duration int, performer, title string, markup *tg.InlineKeyboardMarkup) (*tg.Message, error)
	SendAudioFile(ctx context.Context, chatID int64, path string, thumbPath string, caption string, parseMode string, duration int, performer, title string, markup *tg.InlineKeyboardMarkup) (*tg.Message, error)
	SendCachedFile(ctx context.Context, chatID int64, reference string, caption string, markup *tg.InlineKeyboardMarkup) (*tg.Message, error)
	GetChatMember(ctx context.Context, chatID, userID int64) (*tg.ChatMember, error)
	DeleteMessage(ctx context.Context, chatID, messageID int64) error
}

// Deps bundles every dependency of the handlers.
type Deps struct {
	Config     *config.Config
	Log        *logger.Logger
	DB         *storage.DB
	TG         Telegram
	Bot        *tg.User
	Flow       *media.Flow
	Queue      *download.Queue
	Downloader *download.Downloader
	Stats      *runtime.Stats
	Dedupe     *runtime.Dedupe
	Limiter    *runtime.RateLimiter
	State      *runtime.StateStore
	Services   *services.Registry
	Analytics  *AnalyticsRecorder
}

// Handler processes the incoming updates.
type Handler struct {
	deps Deps
}

// New builds a handler.
func New(deps Deps) *Handler { return &Handler{deps: deps} }

// Handle processes a single update.
func (h *Handler) Handle(ctx context.Context, update tg.Update) {
	switch {
	case update.Message != nil:
		h.handleMessage(ctx, update.Message)
	case update.CallbackQuery != nil:
		h.handleCallback(ctx, update.CallbackQuery)
	case update.InlineQuery != nil:
		h.handleInlineQuery(ctx, update.InlineQuery)
	}
}

// IsAdmin reports whether the user is one of the bot administrators.
func (h *Handler) IsAdmin(userID int64) bool {
	if h.deps.Config == nil {
		return false
	}
	for _, id := range h.deps.Config.AdminIDs() {
		if id == userID {
			return true
		}
	}
	return false
}

// botUsername returns the username of the bot without the leading @.
func (h *Handler) botUsername() string {
	if h.deps.Bot == nil {
		return "IotaDownloaderBot"
	}
	return h.deps.Bot.Username
}

// botLink returns the public link of the bot.
func (h *Handler) botLink() string { return "https://t.me/" + h.botUsername() }

// userSettings loads the preference set of a user.
func (h *Handler) userSettings(ctx context.Context, userID int64) settings.Set {
	if h.deps.DB == nil {
		return settings.DefaultSet()
	}
	set, err := h.deps.DB.GetSettings(ctx, userID)
	if err != nil {
		h.logError("load settings", err)
		return settings.DefaultSet()
	}
	return set
}

// ensureUser stores the user in the database and returns the stored row.
func (h *Handler) ensureUser(ctx context.Context, user *tg.User, chat *tg.Chat) *storage.User {
	if h.deps.DB == nil || user == nil {
		return nil
	}
	chatType := "private"
	if chat != nil && chat.Type != "" {
		chatType = chat.Type
	}
	stored, err := h.deps.DB.UpsertUser(ctx, user.ID, user.DisplayName(), user.Username, chatType, user.LanguageCode)
	if err != nil {
		h.logError("upsert user", err)
		return nil
	}
	return stored
}

// logError writes an error to the log.
func (h *Handler) logError(message string, err error) {
	if h.deps.Log == nil {
		return
	}
	h.deps.Log.Error("%s: %v", message, err)
}

// logInfo writes an informational line to the log.
func (h *Handler) logInfo(message string, args ...any) {
	if h.deps.Log == nil {
		return
	}
	h.deps.Log.Info(message, args...)
}

// logPerf writes a performance sample of a step of a download.
func (h *Handler) logPerf(step, service string, started time.Time) {
	if h.deps.Log == nil {
		return
	}
	h.deps.Log.Perf(step, float64(time.Since(started).Milliseconds()), logger.Fields{"service": service})
}

// commandOf returns the command and the arguments of a message.
func commandOf(text string) (string, []string) {
	fields := strings.Fields(strings.TrimSpace(text))
	if len(fields) == 0 {
		return "", nil
	}
	command := strings.ToLower(fields[0])
	command = strings.TrimPrefix(command, "/")
	if index := strings.Index(command, "@"); index >= 0 {
		command = command[:index]
	}
	return command, fields[1:]
}

// mentioned reports whether the bot was mentioned inside a group message.
func (h *Handler) mentioned(message *tg.Message) bool {
	if message == nil || message.Chat == nil || !message.Chat.IsGroup() {
		return false
	}
	username := h.botUsername()
	if username == "" {
		return false
	}
	for _, entity := range message.Entities {
		if entity.Type != "mention" && entity.Type != "text_mention" {
			continue
		}
		if entity.Type == "text_mention" && entity.User != nil {
			if entity.User.ID == h.deps.Bot.ID {
				return true
			}
			continue
		}
		if text := entityText(message.Text, entity); strings.Contains(strings.ToLower(text), "@"+strings.ToLower(username)) {
			return true
		}
	}
	return strings.Contains(strings.ToLower(message.Text), "@"+strings.ToLower(username))
}

// entityText returns the text of a message entity.
func entityText(text string, entity tg.MessageEntity) string {
	runes := []rune(text)
	if entity.Offset < 0 || entity.Offset+entity.Length > len(runes) || entity.Length <= 0 {
		return ""
	}
	return string(runes[entity.Offset : entity.Offset+entity.Length])
}

// reply sends a text message to a chat.
func (h *Handler) reply(ctx context.Context, chatID int64, text string, markup *tg.InlineKeyboardMarkup) error {
	_, err := h.deps.TG.SendMessage(ctx, chatID, text, tg.SendMessageOptions{
		ParseMode:   "HTML",
		ReplyMarkup: markup,
	})
	return err
}

// editText replaces the text of a message.
func (h *Handler) editText(ctx context.Context, chatID, messageID int64, text string, markup *tg.InlineKeyboardMarkup) error {
	_, err := h.deps.TG.EditMessageText(ctx, chatID, messageID, text, "HTML", markup)
	if isNotModified(err) {
		return nil
	}
	return err
}

// editCaption replaces the caption of a message.
func (h *Handler) editCaption(ctx context.Context, chatID, messageID int64, caption string, markup *tg.InlineKeyboardMarkup) error {
	_, err := h.deps.TG.EditMessageCaption(ctx, chatID, messageID, caption, "HTML", markup)
	if isNotModified(err) {
		return nil
	}
	return err
}

// isNotModified reports whether Telegram refused an edit because the new
// content is identical to the current one.
func isNotModified(err error) bool {
	var apiError *tg.APIError
	return errors.As(err, &apiError) && apiError.IsNotModified()
}

// deleteMessage removes a message and reports whether it worked.
func (h *Handler) deleteMessage(ctx context.Context, chatID, messageID int64) bool {
	if err := h.deps.TG.DeleteMessage(ctx, chatID, messageID); err != nil {
		return false
	}
	return true
}

// retryAfter returns the number of seconds a rate limited request has to wait.
func retryAfter(err error) int {
	if err == nil {
		return 0
	}
	var apiErr *tg.APIError
	if ok := asAPIError(err, &apiErr); ok {
		seconds := int(apiErr.RetryAfter().Seconds())
		if seconds < 1 {
			seconds = 1
		}
		return seconds
	}
	return 5
}

// asAPIError unwraps a telegram API error.
func asAPIError(err error, target **tg.APIError) bool {
	for err != nil {
		if apiErr, ok := err.(*tg.APIError); ok {
			*target = apiErr
			return true
		}
		unwrapped, ok := err.(interface{ Unwrap() error })
		if !ok {
			return false
		}
		err = unwrapped.Unwrap()
	}
	return false
}

// statusMessage renders the status text of a running download.
func statusMessage(text string) string {
	return fmt.Sprintf("%s\n%s", text, "")
}

// nowUTC returns the current time in UTC.
func nowUTC() time.Time { return time.Now().UTC() }
