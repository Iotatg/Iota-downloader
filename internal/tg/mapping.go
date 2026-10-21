package tg

import (
	"encoding/json"
	"strconv"
	"strings"

	gogram "github.com/amarnathcjd/gogram/telegram"
)

// fileReference is the MTProto address of a file that lives on the Telegram
// servers. The reference is stored in the cache of the bot, it allows sending
// the file again without downloading it a second time.
type fileReference struct {
	DC   int   `json:"dc"`
	ID   int64 `json:"id"`
	Hash int64 `json:"hash"`
}

// encode turns the reference into the text that the cache stores.
func (r fileReference) encode() string {
	encoded, err := json.Marshal(r)
	if err != nil {
		return ""
	}
	return string(encoded)
}

// decodeReference reads a reference out of the cached text.
//
// An empty result means that the cache holds a Bot API file id, such a file
// cannot be sent over MTProto.
func decodeReference(value string) (fileReference, bool) {
	value = strings.TrimSpace(value)
	if !strings.HasPrefix(value, "{") {
		return fileReference{}, false
	}
	var reference fileReference
	if err := json.Unmarshal([]byte(value), &reference); err != nil {
		return fileReference{}, false
	}
	if reference.ID == 0 {
		return fileReference{}, false
	}
	return reference, true
}

// document builds the Telegram document of the reference.
func (r fileReference) document() *gogram.DocumentObj {
	return &gogram.DocumentObj{
		ID:         r.ID,
		AccessHash: r.Hash,
		DcID:       int32(r.DC),
	}
}

// referenceOf builds the reference of a Telegram document.
func referenceOf(document gogram.Document) (fileReference, bool) {
	object, ok := document.(*gogram.DocumentObj)
	if !ok || object == nil {
		return fileReference{}, false
	}
	return fileReference{DC: int(object.DcID), ID: object.ID, Hash: object.AccessHash}, true
}

// userFrom converts a Telegram user.
func userFrom(user *gogram.UserObj) *User {
	if user == nil {
		return nil
	}
	return &User{
		ID:           user.ID,
		IsBot:        user.Bot,
		FirstName:    user.FirstName,
		LastName:     user.LastName,
		Username:     user.Username,
		LanguageCode: user.LangCode,
	}
}

// chatFromMessage converts the chat of an incoming message.
//
// The kind of the chat is read from the peer of the message, a private chat
// is described by the user, a group by the chat object and a channel by the
// channel object. The identifiers keep the shape of the Bot API, a group and a
// supergroup carry a negative identifier.
func chatFromMessage(message *gogram.NewMessage) *Chat {
	if message == nil || message.Message == nil {
		return nil
	}
	switch peer := message.Message.PeerID.(type) {
	case *gogram.PeerUser:
		chat := &Chat{ID: peer.UserID, Type: "private"}
		if message.Sender != nil {
			chat.FirstName = message.Sender.FirstName
			chat.LastName = message.Sender.LastName
			chat.Username = message.Sender.Username
		}
		return chat
	case *gogram.PeerChat:
		chat := &Chat{ID: -peer.ChatID, Type: "group"}
		if message.Chat != nil {
			chat.Title = message.Chat.Title
		}
		return chat
	case *gogram.PeerChannel:
		chat := &Chat{ID: channelChatID(peer.ChannelID), Type: "channel"}
		if message.Channel != nil {
			chat.Title = message.Channel.Title
			if message.Channel.Megagroup {
				chat.Type = "supergroup"
			}
		}
		return chat
	}
	return nil
}

// channelChatID builds the identifier of a channel in the shape of the Bot
// API, the prefix keeps the identifier apart from the identifier of a user.
func channelChatID(channelID int64) int64 {
	return -1000000000000 - channelID
}

// chatTypeOf returns the type of the conversation of a message.
func chatTypeOf(message *gogram.NewMessage) string {
	switch {
	case message == nil:
		return ""
	case message.Chat != nil:
		return "group"
	case message.Channel != nil:
		return "channel"
	case message.Sender != nil:
		return "private"
	default:
		return ""
	}
}

// entitiesFrom converts the formatting entities of a message.
func entitiesFrom(entities []gogram.MessageEntity) []MessageEntity {
	if len(entities) == 0 {
		return nil
	}
	converted := make([]MessageEntity, 0, len(entities))
	for _, entity := range entities {
		switch typed := entity.(type) {
		case *gogram.MessageEntityMention:
			converted = append(converted, MessageEntity{Type: "mention", Offset: int(typed.Offset), Length: int(typed.Length)})
		case *gogram.MessageEntityMentionName:
			converted = append(converted, MessageEntity{
				Type:   "text_mention",
				Offset: int(typed.Offset),
				Length: int(typed.Length),
				User:   &User{ID: typed.UserID},
			})
		case *gogram.MessageEntityTextURL:
			converted = append(converted, MessageEntity{Type: "text_link", Offset: int(typed.Offset), Length: int(typed.Length), URL: typed.URL})
		case *gogram.MessageEntityBold:
			converted = append(converted, MessageEntity{Type: "bold", Offset: int(typed.Offset), Length: int(typed.Length)})
		case *gogram.MessageEntityItalic:
			converted = append(converted, MessageEntity{Type: "italic", Offset: int(typed.Offset), Length: int(typed.Length)})
		case *gogram.MessageEntityCode:
			converted = append(converted, MessageEntity{Type: "code", Offset: int(typed.Offset), Length: int(typed.Length)})
		case *gogram.MessageEntityPre:
			converted = append(converted, MessageEntity{Type: "pre", Offset: int(typed.Offset), Length: int(typed.Length), Language: typed.Language})
		}
	}
	if len(converted) == 0 {
		return nil
	}
	return converted
}

// messageFrom converts an incoming message.
func messageFrom(message *gogram.NewMessage) *Message {
	if message == nil || message.Message == nil {
		return nil
	}

	converted := &Message{
		MessageID: int64(message.ID),
		From:      userFrom(message.Sender),
		Chat:      chatFromMessage(message),
		Date:      int64(message.Date()),
	}
	if message.Message.Message != "" {
		converted.Text = message.Message.Message
		converted.Caption = message.Message.Message
	}
	converted.Entities = entitiesFrom(message.Message.Entities)
	converted.CaptionEntities = converted.Entities
	applyMedia(converted, message)
	return converted
}

// applyMedia fills the media fields of a converted message.
func applyMedia(converted *Message, message *gogram.NewMessage) {
	if message == nil || message.Message == nil {
		return
	}
	switch media := message.Message.Media.(type) {
	case *gogram.MessageMediaDocument:
		document, ok := media.Document.(*gogram.DocumentObj)
		if !ok || document == nil {
			return
		}
		reference, _ := referenceOf(document)
		fileID := reference.encode()
		name := fileNameOf(document)
		switch {
		case media.Video:
			converted.Video = &Video{
				FileID:   fileID,
				FileName: name,
				MimeType: document.MimeType,
				FileSize: document.Size,
			}
			applyVideoAttributes(converted.Video, document)
		case isAudioDocument(document):
			converted.Audio = &Audio{
				FileID:   fileID,
				FileName: name,
				MimeType: document.MimeType,
				FileSize: document.Size,
			}
			applyAudioAttributes(converted.Audio, document)
		default:
			converted.Document = &Document{
				FileID:   fileID,
				FileName: name,
				MimeType: document.MimeType,
				FileSize: document.Size,
			}
		}
	case *gogram.MessageMediaPhoto:
		photo, ok := media.Photo.(*gogram.PhotoObj)
		if !ok || photo == nil {
			return
		}
		size := PhotoSize{FileID: strconv.FormatInt(photo.ID, 10)}
		for _, candidate := range photo.Sizes {
			object, ok := candidate.(*gogram.PhotoSizeObj)
			if !ok || object == nil {
				continue
			}
			if int(object.W) > size.Width {
				size.Width = int(object.W)
				size.Height = int(object.H)
				size.FileSize = int64(object.Size)
			}
		}
		converted.Photo = []PhotoSize{size}
	}
}

// applyVideoAttributes copies the size and the duration of a video.
func applyVideoAttributes(video *Video, document *gogram.DocumentObj) {
	for _, attribute := range document.Attributes {
		if typed, ok := attribute.(*gogram.DocumentAttributeVideo); ok {
			video.Width = int(typed.W)
			video.Height = int(typed.H)
			video.Duration = int(typed.Duration)
		}
	}
}

// applyAudioAttributes copies the duration, the title and the performer of an
// audio file.
func applyAudioAttributes(audio *Audio, document *gogram.DocumentObj) {
	for _, attribute := range document.Attributes {
		typed, ok := attribute.(*gogram.DocumentAttributeAudio)
		if !ok {
			continue
		}
		audio.Duration = int(typed.Duration)
		audio.Title = typed.Title
		audio.Performer = typed.Performer
	}
}

// fileNameOf reads the file name of a document.
func fileNameOf(document *gogram.DocumentObj) string {
	for _, attribute := range document.Attributes {
		if typed, ok := attribute.(*gogram.DocumentAttributeFilename); ok {
			return typed.FileName
		}
	}
	return ""
}

// isAudioDocument reports whether a document holds an audio track.
func isAudioDocument(document *gogram.DocumentObj) bool {
	for _, attribute := range document.Attributes {
		if _, ok := attribute.(*gogram.DocumentAttributeAudio); ok {
			return true
		}
	}
	return false
}

// buttonStyle converts the style of a button into the style of the protocol.
//
// The protocol and the Bot API know the same three colours, a button without
// a style keeps the colour of the client.
func buttonStyle(style string) *gogram.KeyboardButtonStyle {
	switch strings.ToLower(style) {
	case ButtonStyleDanger:
		return &gogram.KeyboardButtonStyle{BgDanger: true}
	case ButtonStyleSuccess:
		return &gogram.KeyboardButtonStyle{BgSuccess: true}
	case ButtonStylePrimary:
		return &gogram.KeyboardButtonStyle{BgPrimary: true}
	default:
		return nil
	}
}

// markupFrom converts an inline keyboard.
func markupFrom(markup *InlineKeyboardMarkup) gogram.ReplyMarkup {
	if markup == nil || len(markup.InlineKeyboard) == 0 {
		return nil
	}
	builder := gogram.NewKeyboard()
	for _, row := range markup.InlineKeyboard {
		buttons := make([]gogram.KeyboardButton, 0, len(row))
		for _, button := range row {
			switch {
			case button.URL != "":
				link := gogram.Button.URL(button.Text, button.URL)
				link.Style = buttonStyle(button.Style)
				buttons = append(buttons, link)
			case button.CallbackData != "":
				callback := gogram.Button.Data(button.Text, button.CallbackData)
				callback.Style = buttonStyle(button.Style)
				buttons = append(buttons, callback)
			default:
				plain := gogram.Button.Text(button.Text)
				plain.Style = buttonStyle(button.Style)
				buttons = append(buttons, plain)
			}
		}
		if len(buttons) > 0 {
			builder.AddRow(buttons...)
		}
	}
	return builder.Build()
}

// replyMarkupFrom converts a reply keyboard.
func replyMarkupFrom(markup *ReplyKeyboardMarkup) gogram.ReplyMarkup {
	if markup == nil || len(markup.Keyboard) == 0 {
		return nil
	}
	builder := gogram.NewKeyboard()
	for _, row := range markup.Keyboard {
		buttons := make([]gogram.KeyboardButton, 0, len(row))
		for _, button := range row {
			plain := gogram.Button.Text(button.Text)
			plain.Style = buttonStyle(button.Style)
			buttons = append(buttons, plain)
		}
		if len(buttons) > 0 {
			builder.AddRow(buttons...)
		}
	}
	return builder.BuildReply(gogram.BuildReplyOptions{
		ResizeKeyboard: markup.ResizeKeyboard,
		OneTime:        markup.OneTimeKeyboard,
		Selective:      markup.Selective,
		Placeholder:    markup.InputFieldPlaceholder,
	})
}

// inlineResultsFrom converts the results of an inline query.
func inlineResultsFrom(results []InlineQueryResult) []gogram.InputBotInlineResult {
	converted := make([]gogram.InputBotInlineResult, 0, len(results))
	for _, result := range results {
		article := &gogram.InputBotInlineResultObj{
			ID:          result.ID,
			Type:        "article",
			Title:       result.Title,
			Description: result.Description,
			SendMessage: inputMessageFrom(result.InputMessageContent, result.ReplyMarkup),
		}
		if result.ThumbURL != "" {
			article.Thumb = &gogram.InputWebDocument{
				URL:      result.ThumbURL,
				MimeType: "image/jpeg",
			}
		}
		converted = append(converted, article)
	}
	return converted
}

// inputMessageFrom converts the content of an inline result.
//
// The answer of an inline query carries no parse mode, the styles of the text
// are read here and sent as entities.
func inputMessageFrom(content *InputTextMessageContent, markup *InlineKeyboardMarkup) gogram.InputBotInlineMessage {
	if content == nil {
		return nil
	}
	text, entities := parseHTMLText(content.MessageText)
	return &gogram.InputBotInlineMessageText{
		Message:     text,
		Entities:    entities,
		NoWebpage:   content.DisableWebPagePreview,
		ReplyMarkup: markupFrom(markup),
	}
}

// parseHTMLText reads the styles of an html text and returns the plain text
// and the entities of the Telegram client.
func parseHTMLText(text string) (string, []gogram.MessageEntity) {
	cleaned, tags, err := gogram.ParseHTMLToTags(text)
	if err != nil {
		return text, nil
	}
	entities := make([]gogram.MessageEntity, 0, len(tags))
	for _, tag := range tags {
		switch tag.Type {
		case "b", "strong":
			entities = append(entities, &gogram.MessageEntityBold{Offset: tag.Offset, Length: tag.Length})
		case "i", "em":
			entities = append(entities, &gogram.MessageEntityItalic{Offset: tag.Offset, Length: tag.Length})
		case "u":
			entities = append(entities, &gogram.MessageEntityUnderline{Offset: tag.Offset, Length: tag.Length})
		case "s", "strike", "del":
			entities = append(entities, &gogram.MessageEntityStrike{Offset: tag.Offset, Length: tag.Length})
		case "code":
			entities = append(entities, &gogram.MessageEntityCode{Offset: tag.Offset, Length: tag.Length})
		case "pre":
			entities = append(entities, &gogram.MessageEntityPre{Offset: tag.Offset, Length: tag.Length, Language: tag.Attrs["language"]})
		case "spoiler":
			entities = append(entities, &gogram.MessageEntitySpoiler{Offset: tag.Offset, Length: tag.Length})
		case "mention":
			entities = append(entities, &gogram.MessageEntityMention{Offset: tag.Offset, Length: tag.Length})
		case "a":
			entities = append(entities, &gogram.MessageEntityTextURL{Offset: tag.Offset, Length: tag.Length, URL: tag.Attrs["href"]})
		}
	}
	return cleaned, entities
}

// errorFrom converts a Telegram error into the error of the package.
func errorFrom(err error) error {
	if err == nil {
		return nil
	}
	if _, ok := err.(*APIError); ok {
		return err
	}
	return &APIError{Method: "mtproto", Description: err.Error()}
}
