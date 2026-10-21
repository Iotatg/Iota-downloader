package handlers

import (
	"context"
	"fmt"
	"strings"

	"github.com/Iotatg/iota-downloader/internal/chart"
	"github.com/Iotatg/iota-downloader/internal/keyboards"
	"github.com/Iotatg/iota-downloader/internal/settings"
	"github.com/Iotatg/iota-downloader/internal/tg"
)

// handleCallback routes a button press.
func (h *Handler) handleCallback(ctx context.Context, query *tg.CallbackQuery) {
	if query == nil || query.Message == nil {
		return
	}

	if h.deps.Limiter != nil {
		key := "cb:" + itoa(query.From.ID)
		if !h.deps.Limiter.Allow(key) {
			_ = h.deps.TG.AnswerCallbackQuery(ctx, query.ID, "Slow down a little.", false)
			return
		}
	}

	data := query.Data
	switch {
	case strings.HasPrefix(data, keyboards.PrefixSettings),
		strings.HasPrefix(data, keyboards.PrefixSetting),
		strings.HasPrefix(data, keyboards.PrefixSettingsCat),
		data == "back_to_settings":
		h.handleSettingsCallback(ctx, query)

	case strings.HasPrefix(data, keyboards.PrefixStats):
		h.handleStatsCallback(ctx, query)

	case strings.HasPrefix(data, keyboards.PrefixAdmin),
		strings.HasPrefix(data, "admin_"),
		strings.HasPrefix(data, keyboards.PrefixMailing),
		data == keyboards.AdminOps,
		data == keyboards.AdminBack,
		data == keyboards.AdminNoop,
		data == keyboards.AdminCancel,
		data == keyboards.AdminSendToAll:
		h.handleAdminCallback(ctx, query)

	case strings.HasPrefix(data, keyboards.PrefixInline):
		h.handleInlineCallback(ctx, query)

	case strings.HasPrefix(data, keyboards.PrefixAudio):
		h.handleAudioCallback(ctx, query)

	case strings.HasPrefix(data, keyboards.PrefixDocument):
		h.handleDocumentCallback(ctx, query)

	case strings.HasPrefix(data, keyboards.PrefixCollection):
		h.handleCollectionCallback(ctx, query)

	default:
		_ = h.deps.TG.AnswerCallbackQuery(ctx, query.ID, "", false)
	}
}

// handleStatsCallback switches the period of the statistics screen.
func (h *Handler) handleStatsCallback(ctx context.Context, query *tg.CallbackQuery) {
	parts := strings.Split(strings.TrimPrefix(query.Data, keyboards.PrefixStats), ":")
	period := chart.PeriodWeek
	mode := "total"
	if len(parts) > 0 && parts[0] != "" {
		period = parts[0]
	}
	if len(parts) > 1 && parts[1] != "" {
		mode = parts[1]
	}

	payload, err := h.buildStatsPayload(ctx, period, mode)
	if err != nil {
		h.logError("build stats", err)
		_ = h.deps.TG.AnswerCallbackQuery(ctx, query.ID, "Stats unavailable", true)
		return
	}

	markup := keyboards.StatsKeyboard(payload.Period, payload.Mode)
	caption := statsCaption(payload)
	image, err := h.renderStatsChart(payload)
	if err != nil {
		h.logError("render stats chart", err)
		_ = h.deps.TG.AnswerCallbackQuery(ctx, query.ID, "Stats unavailable", true)
		return
	}

	filename := fmt.Sprintf("stats_%s_%s.png", strings.ToLower(payload.Period), payload.Mode)
	if _, err := h.deps.TG.EditMessageMediaPhoto(ctx, query.Message.Chat.ID, query.Message.MessageID,
		image, filename, caption, "HTML", markup); err != nil {
		h.logError("edit stats chart", err)
		h.replaceStatsMessage(ctx, query, image, filename, caption, markup)
	}
	_ = h.deps.TG.AnswerCallbackQuery(ctx, query.ID, "", false)
}

// replaceStatsMessage sends a new chart when the old message cannot be edited.
func (h *Handler) replaceStatsMessage(ctx context.Context, query *tg.CallbackQuery, image []byte, filename, caption string, markup *tg.InlineKeyboardMarkup) {
	if _, err := h.deps.TG.SendPhotoBytes(ctx, query.Message.Chat.ID, image, filename, caption, "HTML", markup); err != nil {
		h.logError("send stats chart", err)
		return
	}
	h.deleteMessage(ctx, query.Message.Chat.ID, query.Message.MessageID)
}

// isGroupAdmin reports whether the user is an administrator of the chat.
func (h *Handler) isGroupAdmin(ctx context.Context, chatID, userID int64) (bool, error) {
	if h.IsAdmin(userID) {
		return true, nil
	}
	member, err := h.deps.TG.GetChatMember(ctx, chatID, userID)
	if err != nil {
		return false, err
	}
	return member.IsAdmin(), nil
}

// settingsFieldOf returns the setting that belongs to a callback data.
func settingsFieldOf(data string) string {
	if field, ok := settings.ParseViewCallback(data); ok {
		return field
	}
	if field, _, ok := settings.ParseToggleCallback(data); ok {
		return field
	}
	return ""
}
