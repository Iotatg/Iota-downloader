package handlers

import (
	"context"
	"fmt"
	"strings"

	"github.com/Iotatg/iota-downloader/internal/chart"
	"github.com/Iotatg/iota-downloader/internal/keyboards"
	"github.com/Iotatg/iota-downloader/internal/messages"
	"github.com/Iotatg/iota-downloader/internal/settings"
	"github.com/Iotatg/iota-downloader/internal/tg"
)

// handleSettings opens the settings menu.
func (h *Handler) handleSettings(ctx context.Context, message *tg.Message) {
	if h.deps.Analytics != nil {
		h.deps.Analytics.Record("command_settings", message.From.ID, message.Chat.Type)
	}

	if !message.Chat.IsPrivate() {
		allowed, err := h.isGroupAdmin(ctx, message.Chat.ID, message.From.ID)
		if err == nil && allowed {
			if err := h.reply(ctx, message.Chat.ID, messages.SettingsPrivateOnly(), nil); err != nil {
				h.logError("send settings notice", err)
			}
			return
		}
		if err := h.reply(ctx, message.Chat.ID, messages.SettingsAdminOnly(), nil); err != nil {
			h.logError("send settings notice", err)
		}
		return
	}

	set := h.userSettings(ctx, message.From.ID)
	if err := h.reply(ctx, message.Chat.ID, messages.Settings(), keyboards.SettingsKeyboard()); err != nil {
		h.logError("send settings", err)
	}
	_ = set
}

// handleSettingsCallback processes a settings related button press.
func (h *Handler) handleSettingsCallback(ctx context.Context, query *tg.CallbackQuery) {
	userID := query.From.ID
	set := h.userSettings(ctx, userID)

	switch {
	case strings.HasPrefix(query.Data, settings.PrefixCategory):
		category := strings.TrimPrefix(query.Data, settings.PrefixCategory)
		if err := h.editText(ctx, query.Message.Chat.ID, query.Message.MessageID,
			messages.CategorySettingsText(category), keyboards.CategoryKeyboard(category, set)); err != nil {
			h.logError("edit settings category", err)
		}
		return

	case strings.HasPrefix(query.Data, settings.PrefixView):
		field := strings.TrimPrefix(query.Data, settings.PrefixView)
		var markup *tg.InlineKeyboardMarkup
		switch field {
		case settings.FieldVideoQuality:
			markup = keyboards.VideoQualityKeyboard(set.Get(field))
		case settings.FieldAudioFormat:
			markup = keyboards.AudioFormatKeyboard(set.Get(field))
		default:
			markup = keyboards.ToggleKeyboard(field, set.Get(field))
		}
		if err := h.editText(ctx, query.Message.Chat.ID, query.Message.MessageID,
			messages.FieldText(field), markup); err != nil {
			h.logError("edit settings field", err)
		}
		return

	case strings.HasPrefix(query.Data, settings.PrefixToggle):
		parts := strings.Split(strings.TrimPrefix(query.Data, settings.PrefixToggle), ":")
		if len(parts) != 2 {
			break
		}
		field, value := parts[0], parts[1]
		if !settings.IsKnownField(field) || !settings.IsKnownValue(field, value) {
			break
		}
		if err := h.deps.DB.SetSetting(ctx, userID, field, value); err != nil {
			h.logError("store setting", err)
		}
		set = h.userSettings(ctx, userID)
		var markup *tg.InlineKeyboardMarkup
		switch field {
		case settings.FieldVideoQuality:
			markup = keyboards.VideoQualityKeyboard(set.Get(field))
		case settings.FieldAudioFormat:
			markup = keyboards.AudioFormatKeyboard(set.Get(field))
		default:
			markup = keyboards.ToggleKeyboard(field, set.Get(field))
		}
		if err := h.editText(ctx, query.Message.Chat.ID, query.Message.MessageID,
			messages.FieldText(field), markup); err != nil {
			h.logError("edit settings field", err)
		}
		return
	}

	if query.Data == "back_to_settings" {
		if err := h.editText(ctx, query.Message.Chat.ID, query.Message.MessageID,
			messages.Settings(), keyboards.SettingsKeyboard()); err != nil {
			h.logError("edit settings", err)
		}
	}
}

// handleStats renders the statistics screen.
func (h *Handler) handleStats(ctx context.Context, message *tg.Message) {
	if h.deps.Analytics != nil {
		h.deps.Analytics.Record("command_stats", message.From.ID, message.Chat.Type)
	}
	payload, err := h.buildStatsPayload(ctx, chart.PeriodWeek, "total")
	if err != nil {
		h.logError("build stats", err)
		if replyErr := h.reply(ctx, message.Chat.ID, messages.StatsTemporarilyUnavailable(), nil); replyErr != nil {
			h.logError("send stats error", replyErr)
		}
		return
	}

	markup := keyboards.StatsKeyboard(payload.Period, payload.Mode)
	caption := statsCaption(payload)
	image, err := h.renderStatsChart(payload)
	if err != nil {
		h.logError("render stats chart", err)
		if replyErr := h.reply(ctx, message.Chat.ID, caption, markup); replyErr != nil {
			h.logError("send stats error", replyErr)
		}
		return
	}

	filename := fmt.Sprintf("stats_%s_%s.png", strings.ToLower(payload.Period), payload.Mode)
	if _, err := h.deps.TG.SendPhotoBytes(ctx, message.Chat.ID, image, filename, caption, "HTML", markup); err != nil {
		h.logError("send stats chart", err)
		if replyErr := h.reply(ctx, message.Chat.ID, caption, markup); replyErr != nil {
			h.logError("send stats error", replyErr)
		}
	}
}
