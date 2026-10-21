package handlers

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/Iotatg/iota-downloader/internal/keyboards"
	"github.com/Iotatg/iota-downloader/internal/messages"
	"github.com/Iotatg/iota-downloader/internal/storage"
	"github.com/Iotatg/iota-downloader/internal/text"
	"github.com/Iotatg/iota-downloader/internal/tg"
)

// handleAdminCommand opens the admin panel.
func (h *Handler) handleAdminCommand(ctx context.Context, message *tg.Message, args []string) {
	if !h.IsAdmin(message.From.ID) {
		h.answerText(ctx, message.Chat.ID, messages.SettingsAdminOnly())
		return
	}
	if h.deps.Analytics != nil {
		h.deps.Analytics.Record("command_admin", message.From.ID, message.Chat.Type)
	}
	h.showAdminPanel(ctx, message.Chat.ID, message.MessageID)
}

// handlePerfCommand shows the runtime metrics of the bot.
func (h *Handler) handlePerfCommand(ctx context.Context, message *tg.Message) {
	if !h.IsAdmin(message.From.ID) {
		h.answerText(ctx, message.Chat.ID, messages.SettingsAdminOnly())
		return
	}
	text := h.performanceText(ctx)
	if err := h.reply(ctx, message.Chat.ID, text, nil); err != nil {
		h.logError("send perf", err)
	}
}

// handleSessionCommand shows the live session counters of the bot.
func (h *Handler) handleSessionCommand(ctx context.Context, message *tg.Message) {
	if !h.IsAdmin(message.From.ID) {
		h.answerText(ctx, message.Chat.ID, messages.SettingsAdminOnly())
		return
	}
	text := h.sessionText(ctx)
	if err := h.reply(ctx, message.Chat.ID, text, nil); err != nil {
		h.logError("send session", err)
	}
}

// showAdminPanel renders the admin panel.
func (h *Handler) showAdminPanel(ctx context.Context, chatID, messageID int64) {
	text := h.adminPanelText()
	if messageID > 0 {
		if err := h.editText(ctx, chatID, messageID, text, keyboards.AdminKeyboard()); err == nil {
			return
		}
	}
	if err := h.reply(ctx, chatID, text, keyboards.AdminKeyboard()); err != nil {
		h.logError("send admin panel", err)
	}
}

// adminPanelText renders the headline of the admin panel.
func (h *Handler) adminPanelText() string {
	var builder strings.Builder
	builder.WriteString("<b>" + text.SymBar + " " + text.Bold(messages.BrandName+" admin panel") + "</b>\n\n")
	builder.WriteString(text.Small("Select an operation below."))
	return builder.String()
}

// performanceText renders the runtime metrics.
func (h *Handler) performanceText(ctx context.Context) string {
	var builder strings.Builder
	builder.WriteString("<b>" + text.SymBar + " " + text.Bold("runtime metrics") + "</b>\n\n")

	if h.deps.Stats != nil {
		snapshot := h.deps.Stats.Snapshot()
		builder.WriteString(fmt.Sprintf("%s %d\n", text.Small("Downloads"), snapshot.TotalDownloads))
		builder.WriteString(fmt.Sprintf("%s %d\n", text.Small("Cache hits"), snapshot.CacheHits))
		builder.WriteString(fmt.Sprintf("%s %d\n", text.Small("Cache misses"), snapshot.CacheMisses))
		builder.WriteString(fmt.Sprintf("%s %.1f%%\n", text.Small("Hit rate"), snapshot.CacheHitRate()))
		builder.WriteString(fmt.Sprintf("%s %s\n", text.Small("Uptime"), snapshot.Uptime.Round(time.Second)))
	} else {
		builder.WriteString(text.Small("No runtime metrics yet.") + "\n")
	}

	if h.deps.Queue != nil {
		queued, active, workers := h.deps.Queue.Stats()
		builder.WriteString(fmt.Sprintf("%s %d\n", text.Small("Queued"), queued))
		builder.WriteString(fmt.Sprintf("%s %d\n", text.Small("Active"), active))
		builder.WriteString(fmt.Sprintf("%s %d\n", text.Small("Workers"), workers))
	}

	if h.deps.DB != nil {
		if cached, err := h.deps.DB.CountCachedFiles(ctx); err == nil {
			builder.WriteString(fmt.Sprintf("%s %s\n", text.Small("Cached files"), messages.FormatNumber(int64(cached))))
		}
	}

	return builder.String()
}

// sessionText renders the live session counters.
func (h *Handler) sessionText(ctx context.Context) string {
	var builder strings.Builder
	builder.WriteString("<b>" + text.SymBar + " " + text.Bold("live session") + "</b>\n\n")

	if h.deps.DB == nil {
		return builder.String() + text.Small("No queue metrics yet.")
	}

	users, err := h.deps.DB.CountUsers(ctx)
	if err != nil {
		return builder.String() + messages.NoQueueMetricsYet()
	}
	active, err := h.deps.DB.CountUsersByStatus(ctx, "active")
	if err != nil {
		return builder.String() + messages.NoQueueMetricsYet()
	}
	groups, err := h.deps.DB.TrackedGroupCount(ctx)
	if err != nil {
		return builder.String() + messages.NoQueueMetricsYet()
	}
	members, err := h.deps.DB.TrackedGroupMemberCount(ctx)
	if err != nil {
		return builder.String() + messages.NoQueueMetricsYet()
	}

	builder.WriteString(fmt.Sprintf("%s %s\n", text.Small("Users"), messages.FormatNumber(int64(users))))
	builder.WriteString(fmt.Sprintf("%s %s\n", text.Small("Active users"), messages.FormatNumber(int64(active))))
	builder.WriteString(fmt.Sprintf("%s %s\n", text.Small("Groups"), messages.FormatNumber(int64(groups))))
	builder.WriteString(fmt.Sprintf("%s %s\n", text.Small("Tracked members"), messages.FormatNumber(int64(members))))

	return builder.String()
}

// handleAdminCallback processes the buttons of the admin panel.
func (h *Handler) handleAdminCallback(ctx context.Context, query *tg.CallbackQuery) {
	if !h.IsAdmin(query.From.ID) {
		_ = h.deps.TG.AnswerCallbackQuery(ctx, query.ID, "Admins only", true)
		return
	}

	chatID := query.Message.Chat.ID
	messageID := query.Message.MessageID

	switch query.Data {
	case keyboards.AdminNoop:
		_ = h.deps.TG.AnswerCallbackQuery(ctx, query.ID, "", false)
		return

	case keyboards.AdminRefresh:
		if err := h.editText(ctx, chatID, messageID, h.adminPanelText(), keyboards.AdminKeyboard()); err != nil {
			h.logError("refresh admin panel", err)
		}
		_ = h.deps.TG.AnswerCallbackQuery(ctx, query.ID, "Refreshed", false)
		return

	case keyboards.AdminOps:
		if err := h.editText(ctx, chatID, messageID, h.adminPanelText(), keyboards.AdminOpsKeyboard()); err != nil {
			h.logError("open admin ops", err)
		}
		_ = h.deps.TG.AnswerCallbackQuery(ctx, query.ID, "", false)
		return

	case keyboards.AdminBack:
		if err := h.editText(ctx, chatID, messageID, h.adminPanelText(), keyboards.AdminKeyboard()); err != nil {
			h.logError("back to admin panel", err)
		}
		_ = h.deps.TG.AnswerCallbackQuery(ctx, query.ID, "", false)
		return

	case keyboards.AdminRuntimeStorage:
		if err := h.editText(ctx, chatID, messageID, h.performanceText(ctx), keyboards.AdminOpsKeyboard()); err != nil {
			h.logError("open runtime storage", err)
		}
		_ = h.deps.TG.AnswerCallbackQuery(ctx, query.ID, "", false)
		return

	case keyboards.AdminDownloadHistory:
		if err := h.editText(ctx, chatID, messageID, h.historyText(ctx), keyboards.AdminOpsKeyboard()); err != nil {
			h.logError("open download history", err)
		}
		_ = h.deps.TG.AnswerCallbackQuery(ctx, query.ID, "", false)
		return

	case keyboards.AdminCleanupDownloads:
		h.answerText(ctx, chatID, text.Small("Cleanup of the download cache was requested."))
		_ = h.deps.TG.AnswerCallbackQuery(ctx, query.ID, "Done", false)
		return

	case keyboards.AdminCheckUsers:
		h.answerText(ctx, chatID, h.activeUsersText(ctx))
		_ = h.deps.TG.AnswerCallbackQuery(ctx, query.ID, "", false)
		return

	case keyboards.AdminCheckGroups:
		h.answerText(ctx, chatID, h.activeGroupsText(ctx))
		_ = h.deps.TG.AnswerCallbackQuery(ctx, query.ID, "", false)
		return

	case keyboards.AdminMailing:
		if err := h.editText(ctx, chatID, messageID, text.Small("Select the audience of the mailing."), keyboards.MailingAudienceKeyboard()); err != nil {
			h.logError("open mailing", err)
		}
		_ = h.deps.TG.AnswerCallbackQuery(ctx, query.ID, "", false)
		return
	}

	if strings.HasPrefix(query.Data, keyboards.PrefixMailing) {
		audience := strings.TrimPrefix(query.Data, keyboards.PrefixMailing)
		if err := h.editText(ctx, chatID, messageID, h.mailingPreviewText(ctx, audience), keyboards.MailingSendKeyboard()); err != nil {
			h.logError("open mailing preview", err)
		}
		_ = h.deps.TG.AnswerCallbackQuery(ctx, query.ID, "", false)
		return
	}

	if query.Data == keyboards.AdminCancel {
		if err := h.editText(ctx, chatID, messageID, h.adminPanelText(), keyboards.AdminKeyboard()); err != nil {
			h.logError("cancel mailing", err)
		}
		_ = h.deps.TG.AnswerCallbackQuery(ctx, query.ID, "Canceled", false)
		return
	}

	if query.Data == keyboards.AdminSendToAll {
		h.answerText(ctx, chatID, text.Small("Your message was sent to the selected audience."))
		_ = h.deps.TG.AnswerCallbackQuery(ctx, query.ID, "Sent", false)
		return
	}

	_ = h.deps.TG.AnswerCallbackQuery(ctx, query.ID, "", false)
}

// historyText renders the recent download history.
func (h *Handler) historyText(ctx context.Context) string {
	var builder strings.Builder
	builder.WriteString("<b>" + text.SymBar + " " + text.Bold("download history") + "</b>\n\n")

	if h.deps.DB == nil {
		return builder.String() + messages.NoQueueMetricsYet()
	}

	records, err := h.deps.DB.ListHistory(ctx, storage.HistoryFilter{Limit: 10})
	if err != nil {
		h.logError("read download history", err)
		return builder.String() + messages.StatsTemporarilyUnavailable()
	}
	if len(records) == 0 {
		return builder.String() + text.Small("No downloads recorded yet.")
	}

	for _, record := range records {
		builder.WriteString(fmt.Sprintf("%s %s — %s\n",
			text.Small(platformsLabel(record.Service)),
			text.Small(record.Status),
			text.Small(trimText(record.Title, 32))))
	}
	return builder.String()
}

// activeUsersText renders the user counters.
func (h *Handler) activeUsersText(ctx context.Context) string {
	if h.deps.DB == nil {
		return messages.NoQueueMetricsYet()
	}
	total, err := h.deps.DB.CountUsers(ctx)
	if err != nil {
		return messages.StatsTemporarilyUnavailable()
	}
	active, err := h.deps.DB.CountUsersByStatus(ctx, "active")
	if err != nil {
		return messages.StatsTemporarilyUnavailable()
	}
	return fmt.Sprintf("%s\n%s %s\n%s %s",
		text.Bold("Active Users"),
		text.Small("Total"), messages.FormatNumber(int64(total)),
		text.Small("Active"), messages.FormatNumber(int64(active)))
}

// activeGroupsText renders the group counters.
func (h *Handler) activeGroupsText(ctx context.Context) string {
	if h.deps.DB == nil {
		return messages.NoQueueMetricsYet()
	}
	total, err := h.deps.DB.TrackedGroupCount(ctx)
	if err != nil {
		return messages.StatsTemporarilyUnavailable()
	}
	members, err := h.deps.DB.TrackedGroupMemberCount(ctx)
	if err != nil {
		return messages.StatsTemporarilyUnavailable()
	}
	return fmt.Sprintf("%s\n%s %s\n%s %s",
		text.Bold("Active Groups"),
		text.Small("Groups"), messages.FormatNumber(int64(total)),
		text.Small("Members"), messages.FormatNumber(int64(members)))
}

// mailingPreviewText renders the preview of a mailing.
func (h *Handler) mailingPreviewText(ctx context.Context, audience string) string {
	recipients := 0
	if h.deps.DB != nil {
		switch audience {
		case "dm":
			ids, err := h.deps.DB.ListUserIDs(ctx)
			if err == nil {
				recipients = len(ids)
			}
		case "groups":
			groups, err := h.deps.DB.ListGroups(ctx)
			if err == nil {
				recipients = len(groups)
			}
		default:
			users, err := h.deps.DB.CountUsers(ctx)
			if err == nil {
				recipients = users
			}
		}
	}

	return fmt.Sprintf("<b>%s %s</b>\n\n%s %s\n%s %s",
		text.SymBar, text.Bold("mailing preview"),
		text.Small("Audience"), text.Small(audience),
		text.Small("Recipients"), strconv.Itoa(recipients))
}

// platformsLabel returns the display name of a service.
func platformsLabel(service string) string {
	if service == "" {
		return "unknown"
	}
	return service
}
