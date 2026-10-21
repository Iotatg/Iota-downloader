package handlers

import (
	"context"
	"strings"

	"github.com/Iotatg/iota-downloader/internal/keyboards"
	"github.com/Iotatg/iota-downloader/internal/messages"
	"github.com/Iotatg/iota-downloader/internal/platforms"
	"github.com/Iotatg/iota-downloader/internal/storage"
	"github.com/Iotatg/iota-downloader/internal/tg"
)

// handleMessage routes an incoming message.
func (h *Handler) handleMessage(ctx context.Context, message *tg.Message) {
	if message == nil || message.Chat == nil {
		return
	}

	user := message.From
	if user == nil || user.IsBot {
		return
	}

	if h.deps.Limiter != nil {
		key := messageKey(message)
		if !h.deps.Limiter.Allow(key) {
			h.answerTooMany(ctx, message)
			return
		}
	}

	text := message.Text
	if text == "" {
		text = message.Caption
	}

	if strings.HasPrefix(text, "/") {
		h.handleCommand(ctx, message, text)
		return
	}

	h.ensureUser(ctx, user, message.Chat)

	if message.Chat.IsGroup() {
		h.handleGroupMessage(ctx, message, text)
		return
	}

	if !containsLink(text) {
		return
	}
	h.handlePrivateLink(ctx, message, text)
}

// handleCommand dispatches a bot command.
func (h *Handler) handleCommand(ctx context.Context, message *tg.Message, text string) {
	command, args := commandOf(text)
	if command == "" {
		return
	}

	user := message.From
	h.ensureUser(ctx, user, message.Chat)

	switch command {
	case "start":
		h.handleStart(ctx, message)
	case "help":
		h.handleHelp(ctx, message)
	case "settings":
		h.handleSettings(ctx, message)
	case "stats":
		h.handleStats(ctx, message)
	case "remove_keyboard":
		h.handleRemoveKeyboard(ctx, message)
	case "admin":
		h.handleAdminCommand(ctx, message, args)
	case "perf":
		h.handlePerfCommand(ctx, message)
	case "session":
		h.handleSessionCommand(ctx, message)
	default:
		if h.deps.Analytics != nil {
			h.deps.Analytics.Record("command_unknown", user.ID, message.Chat.Type)
		}
	}
}

// handleStart greets the user.
func (h *Handler) handleStart(ctx context.Context, message *tg.Message) {
	if h.deps.Analytics != nil {
		h.deps.Analytics.Record("command_start", message.From.ID, message.Chat.Type)
	}
	if h.deps.DB != nil {
		_ = h.deps.DB.SetUserHasDM(ctx, message.From.ID, true)
	}

	_, err := h.deps.TG.SendMessage(ctx, message.Chat.ID, messages.Welcome(h.botUsername()), tg.SendMessageOptions{
		ParseMode:     "HTML",
		ReplyKeyboard: keyboards.StartKeyboard(),
	})
	if err != nil {
		h.logError("send welcome", err)
	}
}

// handleHelp explains the usage of the bot.
func (h *Handler) handleHelp(ctx context.Context, message *tg.Message) {
	if h.deps.Analytics != nil {
		h.deps.Analytics.Record("command_help", message.From.ID, message.Chat.Type)
	}
	if err := h.reply(ctx, message.Chat.ID, messages.Help(h.botUsername()), nil); err != nil {
		h.logError("send help", err)
	}
}

// handleRemoveKeyboard hides the reply keyboard.
func (h *Handler) handleRemoveKeyboard(ctx context.Context, message *tg.Message) {
	_, err := h.deps.TG.SendMessage(ctx, message.Chat.ID, messages.KeyboardRemoved(), tg.SendMessageOptions{
		ParseMode:      "HTML",
		RemoveKeyboard: true,
	})
	if err != nil {
		h.logError("remove keyboard", err)
	}
}

// handleGroupMessage reacts to a message inside a group chat.
func (h *Handler) handleGroupMessage(ctx context.Context, message *tg.Message, text string) {
	if !h.mentioned(message) {
		return
	}

	cleaned := removeMention(text, h.botUsername())
	if !containsLink(cleaned) {
		if err := h.reply(ctx, message.Chat.ID, messages.GuestHelpMessage(h.botUsername()), nil); err != nil {
			h.logError("send guest help", err)
		}
		return
	}

	h.handleGuestLink(ctx, message, cleaned)
}

// messageKey builds the anti flood key of a message.
func messageKey(message *tg.Message) string {
	chatID := int64(0)
	if message.Chat != nil {
		chatID = message.Chat.ID
	}
	userID := int64(0)
	if message.From != nil {
		userID = message.From.ID
	}
	return "msg:" + itoa(chatID) + ":" + itoa(userID)
}

// answerTooMany warns a user about the anti flood limits.
func (h *Handler) answerTooMany(ctx context.Context, message *tg.Message) {
	if message.Chat == nil {
		return
	}
	if err := h.reply(ctx, message.Chat.ID, messages.RateLimitText(10), nil); err != nil {
		h.logError("send rate limit notice", err)
	}
}

// containsLink reports whether the text holds a supported link.
func containsLink(text string) bool {
	return len(platforms.ExtractLinks(text)) > 0
}

// removeMention strips the bot mention from a group message.
func removeMention(text, username string) string {
	lower := strings.ToLower(text)
	target := "@" + strings.ToLower(username)
	for {
		index := strings.Index(lower, target)
		if index < 0 {
			break
		}
		text = text[:index] + " " + text[index+len(target):]
		lower = strings.ToLower(text)
	}
	return text
}

// itoa renders an int64.
func itoa(value int64) string {
	if value == 0 {
		return "0"
	}
	negative := value < 0
	if negative {
		value = -value
	}
	digits := make([]byte, 0, 20)
	for value > 0 {
		digits = append(digits, byte('0'+value%10))
		value /= 10
	}
	if negative {
		digits = append(digits, '-')
	}
	for left, right := 0, len(digits)-1; left < right; left, right = left+1, right-1 {
		digits[left], digits[right] = digits[right], digits[left]
	}
	return string(digits)
}

// userStatus maps a user row onto the status label of the statistics screen.
func userStatus(user *storage.User) string {
	if user == nil {
		return "active"
	}
	if user.Status != "" {
		return user.Status
	}
	return "active"
}
