package handlers

import (
	"context"
	"strconv"
	"strings"

	"github.com/Iotatg/iota-downloader/internal/keyboards"
	"github.com/Iotatg/iota-downloader/internal/messages"
	"github.com/Iotatg/iota-downloader/internal/platforms"
	"github.com/Iotatg/iota-downloader/internal/tg"
)

// showCollection expands a collection link and shows the menu of the
// collection.
//
// It reports whether the menu was shown, a platform without a reader of its
// own falls back to the normal delivery of the link.
func (h *Handler) showCollection(ctx context.Context, message *tg.Message, link platforms.Link, ref platforms.CollectionRef) bool {
	if message == nil || message.Chat == nil {
		return false
	}
	if h.deps.Services == nil {
		return false
	}

	limit := h.collectionMaxItems()
	status := h.sendStatus(ctx, message.Chat.ID, messages.ResolvingStatus(platforms.ServiceLabel(ref.Service)))

	collection, err := h.deps.Services.ResolveCollection(ctx, ref, link.URL, limit)
	if err != nil {
		h.logError("expand collection", err)
		if status != nil {
			_ = h.deps.TG.DeleteMessage(ctx, message.Chat.ID, status.MessageID)
		}
		return false
	}
	if collection == nil || len(collection.Items) == 0 {
		if status != nil {
			_ = h.deps.TG.DeleteMessage(ctx, message.Chat.ID, status.MessageID)
		}
		h.answerText(ctx, message.Chat.ID, messages.CollectionEmpty())
		return true
	}

	token := h.deps.Services.Collections.Put(collection)
	if token == "" {
		if status != nil {
			_ = h.deps.TG.DeleteMessage(ctx, message.Chat.ID, status.MessageID)
		}
		return false
	}

	body := messages.CollectionTitle(collection)
	markup := keyboards.CollectionKeyboard(collection, token, 0, h.collectionPageSize())

	if status != nil {
		if _, editErr := h.deps.TG.EditMessageText(ctx, message.Chat.ID, status.MessageID, body, "HTML", markup); editErr == nil {
			return true
		}
		_ = h.deps.TG.DeleteMessage(ctx, message.Chat.ID, status.MessageID)
	}
	if _, err := h.deps.TG.SendMessage(ctx, message.Chat.ID, body, tg.SendMessageOptions{ParseMode: "HTML", ReplyMarkup: markup}); err != nil {
		h.logError("send collection menu", err)
		return false
	}
	return true
}

// collectionMaxItems returns the number of the entries a collection is
// expanded into.
func (h *Handler) collectionMaxItems() int {
	if h.deps.Config == nil || h.deps.Config.CollectionMaxItems <= 0 {
		return 50
	}
	return h.deps.Config.CollectionMaxItems
}

// collectionPageSize returns the number of the entries the menu shows at
// once.
func (h *Handler) collectionPageSize() int {
	if h.deps.Config == nil || h.deps.Config.CollectionPageSize <= 0 {
		return 10
	}
	return h.deps.Config.CollectionPageSize
}

// handleCollectionCallback processes a button of the menu of a collection.
func (h *Handler) handleCollectionCallback(ctx context.Context, query *tg.CallbackQuery) {
	if query == nil || query.Message == nil {
		return
	}
	if h.deps.Services == nil {
		_ = h.deps.TG.AnswerCallbackQuery(ctx, query.ID, "", false)
		return
	}

	token, page, action, ok := parseCollectionCallback(query.Data)
	if !ok {
		_ = h.deps.TG.AnswerCallbackQuery(ctx, query.ID, "", false)
		return
	}

	collection := h.deps.Services.Collections.Get(token)
	if collection == nil {
		_ = h.deps.TG.AnswerCallbackQuery(ctx, query.ID, "the list is too old, send the link again.", true)
		return
	}

	switch action {
	case keyboards.CollectionActionClose:
		_ = h.deps.TG.AnswerCallbackQuery(ctx, query.ID, "", false)
		_ = h.deps.TG.DeleteMessage(ctx, query.Message.Chat.ID, query.Message.MessageID)
		return

	case "p":
		_ = h.deps.TG.AnswerCallbackQuery(ctx, query.ID, "", false)
		body := messages.CollectionTitle(collection)
		markup := keyboards.CollectionKeyboard(collection, token, page, h.collectionPageSize())
		if _, err := h.deps.TG.EditMessageText(ctx, query.Message.Chat.ID, query.Message.MessageID, body, "HTML", markup); err != nil {
			h.logError("edit collection menu", err)
		}
		return

	case keyboards.CollectionActionAll:
		_ = h.deps.TG.AnswerCallbackQuery(ctx, query.ID, "sending the collection...", false)
		h.sendCollection(ctx, query, collection, collection.Items)
		return
	}

	index, convErr := strconv.Atoi(action)
	if convErr != nil || index < 0 || index >= len(collection.Items) {
		_ = h.deps.TG.AnswerCallbackQuery(ctx, query.ID, "", false)
		return
	}

	_ = h.deps.TG.AnswerCallbackQuery(ctx, query.ID, "", false)
	h.sendCollection(ctx, query, collection, []platforms.CollectionItem{collection.Items[index]})
}

// sendCollection delivers the entries of a collection.
//
// The entries are delivered with the normal pipeline of the bot, so a track
// of a playlist is delivered like a link that was sent on its own.
func (h *Handler) sendCollection(ctx context.Context, query *tg.CallbackQuery, collection *platforms.Collection, items []platforms.CollectionItem) {
	if query == nil || query.Message == nil {
		return
	}
	chatID := query.Message.Chat.ID
	total := len(items)

	// The message of the menu was sent by the bot, the entry belongs to the
	// user who pressed the button, so the delivery is made on behalf of that
	// user.
	message := collectionMessageOf(query)
	if message == nil {
		return
	}

	for index, item := range items {
		if item.URL == "" {
			continue
		}
		if total > 1 {
			body := messages.CollectionSending(index+1, total)
			if _, err := h.deps.TG.EditMessageText(ctx, chatID, query.Message.MessageID, body, "HTML", nil); err != nil {
				h.logError("edit collection progress", err)
			}
		}

		h.processLinkEntry(ctx, message, platforms.Link{
			Service: collection.Service,
			URL:     item.URL,
		}, false)
	}

	if total > 1 {
		body := messages.CollectionSent(total)
		if _, err := h.deps.TG.EditMessageText(ctx, chatID, query.Message.MessageID, body, "HTML", nil); err != nil {
			h.logError("edit collection result", err)
		}
	}
}

// collectionMessageOf returns the message a collection entry is delivered
// with.
//
// The menu of the collection was sent by the bot, the entry belongs to the
// user who pressed the button, so the delivery runs under that user.
func collectionMessageOf(query *tg.CallbackQuery) *tg.Message {
	if query == nil || query.Message == nil {
		return nil
	}
	message := *query.Message
	if query.From != nil {
		message.From = query.From
	}
	return &message
}

// parseCollectionCallback reads the token, the page and the action out of
// the callback data of the menu of a collection.
func parseCollectionCallback(data string) (string, int, string, bool) {
	if !strings.HasPrefix(data, keyboards.PrefixCollection) {
		return "", 0, "", false
	}
	parts := strings.Split(strings.TrimPrefix(data, keyboards.PrefixCollection), ":")
	if len(parts) != 3 {
		return "", 0, "", false
	}
	page, err := strconv.Atoi(parts[1])
	if err != nil {
		return "", 0, "", false
	}
	if parts[2] == "" {
		return "", 0, "", false
	}
	return parts[0], page, parts[2], true
}
