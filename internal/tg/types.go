// Package tg is a small Telegram client that speaks MTProto. It covers the
// subset of the Telegram API that the Iota Coder downloader needs, the client
// removes the size limit of the Bot API.
package tg

import (
	"fmt"
	"strings"
	"time"
)

// User represents a Telegram user or bot.
type User struct {
	ID           int64  `json:"id"`
	IsBot        bool   `json:"is_bot"`
	FirstName    string `json:"first_name"`
	LastName     string `json:"last_name"`
	Username     string `json:"username"`
	LanguageCode string `json:"language_code"`
}

// DisplayName returns a human friendly name for the user.
func (u *User) DisplayName() string {
	switch {
	case u == nil:
		return ""
	case u.FirstName != "" && u.LastName != "":
		return u.FirstName + " " + u.LastName
	case u.FirstName != "":
		return u.FirstName
	case u.Username != "":
		return u.Username
	default:
		return fmt.Sprintf("%d", u.ID)
	}
}

// Chat represents a chat the bot talks to.
type Chat struct {
	ID        int64  `json:"id"`
	Type      string `json:"type"`
	Title     string `json:"title"`
	Username  string `json:"username"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
}

// IsPrivate reports whether the chat is a private chat.
func (c *Chat) IsPrivate() bool { return c != nil && c.Type == "private" }

// IsGroup reports whether the chat is a group or supergroup.
func (c *Chat) IsGroup() bool {
	return c != nil && (c.Type == "group" || c.Type == "supergroup")
}

// Name returns the best available display name of the chat.
func (c *Chat) Name() string {
	switch {
	case c == nil:
		return ""
	case c.Title != "":
		return c.Title
	case c.FirstName != "":
		return c.FirstName
	case c.Username != "":
		return c.Username
	default:
		return fmt.Sprintf("%d", c.ID)
	}
}

// MessageEntity marks a styled range inside a text.
type MessageEntity struct {
	Type          string `json:"type"`
	Offset        int    `json:"offset"`
	Length        int    `json:"length"`
	URL           string `json:"url,omitempty"`
	User          *User  `json:"user,omitempty"`
	Language      string `json:"language,omitempty"`
	CustomEmojiID string `json:"custom_emoji_id,omitempty"`
}

// PhotoSize describes one size of a photo.
type PhotoSize struct {
	FileID       string `json:"file_id"`
	FileUniqueID string `json:"file_unique_id"`
	Width        int    `json:"width"`
	Height       int    `json:"height"`
	FileSize     int64  `json:"file_size,omitempty"`
}

// Video describes a video attachment.
type Video struct {
	FileID       string      `json:"file_id"`
	FileUniqueID string      `json:"file_unique_id"`
	Width        int         `json:"width"`
	Height       int         `json:"height"`
	Duration     int         `json:"duration"`
	FileName     string      `json:"file_name,omitempty"`
	MimeType     string      `json:"mime_type,omitempty"`
	FileSize     int64       `json:"file_size,omitempty"`
	Thumbnails   []PhotoSize `json:"thumbnails,omitempty"`
	Thumbnail    *PhotoSize  `json:"thumbnail,omitempty"`
}

// Audio describes an audio attachment.
type Audio struct {
	FileID       string      `json:"file_id"`
	FileUniqueID string      `json:"file_unique_id"`
	Duration     int         `json:"duration"`
	Performer    string      `json:"performer,omitempty"`
	Title        string      `json:"title,omitempty"`
	FileName     string      `json:"file_name,omitempty"`
	MimeType     string      `json:"mime_type,omitempty"`
	FileSize     int64       `json:"file_size,omitempty"`
	Thumbnails   []PhotoSize `json:"thumbnails,omitempty"`
	Thumbnail    *PhotoSize  `json:"thumbnail,omitempty"`
}

// Document describes a document attachment.
type Document struct {
	FileID       string      `json:"file_id"`
	FileUniqueID string      `json:"file_unique_id"`
	FileName     string      `json:"file_name,omitempty"`
	MimeType     string      `json:"mime_type,omitempty"`
	FileSize     int64       `json:"file_size,omitempty"`
	Thumbnails   []PhotoSize `json:"thumbnails,omitempty"`
	Thumbnail    *PhotoSize  `json:"thumbnail,omitempty"`
}

// Animation describes an animation attachment.
type Animation struct {
	FileID       string      `json:"file_id"`
	FileUniqueID string      `json:"file_unique_id"`
	Width        int         `json:"width"`
	Height       int         `json:"height"`
	Duration     int         `json:"duration"`
	FileName     string      `json:"file_name,omitempty"`
	MimeType     string      `json:"mime_type,omitempty"`
	FileSize     int64       `json:"file_size,omitempty"`
	Thumbnails   []PhotoSize `json:"thumbnails,omitempty"`
	Thumbnail    *PhotoSize  `json:"thumbnail,omitempty"`
}

// ReplyParameters describes how a message replies to another one.
type ReplyParameters struct {
	MessageID                int64 `json:"message_id"`
	ChatID                   int64 `json:"chat_id,omitempty"`
	AllowSendingWithoutReply bool  `json:"allow_sending_without_reply,omitempty"`
}

// Message is a Telegram message.
type Message struct {
	MessageID       int64            `json:"message_id"`
	From            *User            `json:"from,omitempty"`
	Chat            *Chat            `json:"chat"`
	Date            int64            `json:"date"`
	Text            string           `json:"text,omitempty"`
	Caption         string           `json:"caption,omitempty"`
	Entities        []MessageEntity  `json:"entities,omitempty"`
	CaptionEntities []MessageEntity  `json:"caption_entities,omitempty"`
	Photo           []PhotoSize      `json:"photo,omitempty"`
	Video           *Video           `json:"video,omitempty"`
	Audio           *Audio           `json:"audio,omitempty"`
	Document        *Document        `json:"document,omitempty"`
	Animation       *Animation       `json:"animation,omitempty"`
	ReplyToMessage  *Message         `json:"reply_to_message,omitempty"`
	ReplyParameters *ReplyParameters `json:"reply_parameters,omitempty"`
	ViaBot          *User            `json:"via_bot,omitempty"`
}

// ChatMember holds the membership state of a user inside a chat.
type ChatMember struct {
	User   *User  `json:"user"`
	Status string `json:"status"`
	Info   struct {
		Title    string `json:"title,omitempty"`
		IsAnonym bool   `json:"is_anonymized,omitempty"`
		CanPost  bool   `json:"can_post_messages,omitempty"`
	} `json:"info"`
}

// IsAdmin reports whether the member is an administrator or the creator.
func (m *ChatMember) IsAdmin() bool {
	return m != nil && (m.Status == "administrator" || m.Status == "creator")
}

// CallbackQuery is a press of an inline keyboard button.
type CallbackQuery struct {
	ID      string   `json:"id"`
	From    *User    `json:"from"`
	Message *Message `json:"message,omitempty"`
	Data    string   `json:"data,omitempty"`
}

// InlineQuery is an inline mode query.
type InlineQuery struct {
	ID       string    `json:"id"`
	From     *User     `json:"from"`
	Query    string    `json:"query"`
	Offset   string    `json:"offset"`
	ChatType string    `json:"chat_type,omitempty"`
	Location *Location `json:"location,omitempty"`
}

// Location is a geographic position attached to an inline query.
type Location struct {
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
}

// ChosenInlineResult is sent when a user picks an inline result.
type ChosenInlineResult struct {
	ResultID        string    `json:"result_id"`
	From            *User     `json:"from"`
	Location        *Location `json:"location,omitempty"`
	InlineMessageID string    `json:"inline_message_id,omitempty"`
	Query           string    `json:"query,omitempty"`
}

// Update is a single incoming update.
type Update struct {
	UpdateID           int64               `json:"update_id"`
	Message            *Message            `json:"message,omitempty"`
	EditedMessage      *Message            `json:"edited_message,omitempty"`
	CallbackQuery      *CallbackQuery      `json:"callback_query,omitempty"`
	InlineQuery        *InlineQuery        `json:"inline_query,omitempty"`
	ChosenInlineResult *ChosenInlineResult `json:"chosen_inline_result,omitempty"`
}

// File describes a file stored on the Telegram servers.
type File struct {
	FileID       string `json:"file_id"`
	FileUniqueID string `json:"file_unique_id"`
	FilePath     string `json:"file_path,omitempty"`
	FileSize     int64  `json:"file_size,omitempty"`
}

// WebhookInfo describes the current webhook state.
type WebhookInfo struct {
	URL                  string `json:"url"`
	HasCustomCertificate bool   `json:"has_custom_certificate"`
	PendingUpdateCount   int    `json:"pending_update_count"`
}

// BotCommand is a single command menu entry.
type BotCommand struct {
	Command     string `json:"command"`
	Description string `json:"description"`
}

// Button styles accepted by the Bot API.
const (
	ButtonStyleDanger  = "danger"
	ButtonStyleSuccess = "success"
	ButtonStylePrimary = "primary"
)

// InlineKeyboardButton is one button of an inline keyboard.
type InlineKeyboardButton struct {
	Text         string      `json:"text"`
	Style        string      `json:"style,omitempty"`
	URL          string      `json:"url,omitempty"`
	CallbackData string      `json:"callback_data,omitempty"`
	WebApp       *WebAppInfo `json:"web_app,omitempty"`
}

// WebAppInfo describes an inline web application button.
type WebAppInfo struct {
	URL string `json:"url"`
}

// InlineKeyboardMarkup is a set of inline keyboard rows.
type InlineKeyboardMarkup struct {
	InlineKeyboard [][]InlineKeyboardButton `json:"inline_keyboard,omitempty"`
}

// KeyboardButton is one button of a reply keyboard.
type KeyboardButton struct {
	Text            string      `json:"text"`
	Style           string      `json:"style,omitempty"`
	RequestContact  bool        `json:"request_contact,omitempty"`
	RequestLocation bool        `json:"request_location,omitempty"`
	WebApp          *WebAppInfo `json:"web_app,omitempty"`
}

// ReplyKeyboardMarkup is a custom reply keyboard.
type ReplyKeyboardMarkup struct {
	Keyboard              [][]KeyboardButton `json:"keyboard"`
	ResizeKeyboard        bool               `json:"resize_keyboard,omitempty"`
	OneTimeKeyboard       bool               `json:"one_time_keyboard,omitempty"`
	InputFieldPlaceholder string             `json:"input_field_placeholder,omitempty"`
	Selective             bool               `json:"selective,omitempty"`
}

// ReplyKeyboardRemove hides the custom keyboard.
type ReplyKeyboardRemove struct {
	RemoveKeyboard bool `json:"remove_keyboard"`
	Selective      bool `json:"selective,omitempty"`
}

// ResponseParameters describes where to send a message.
type ResponseParameters struct {
	ReplyParameters *ReplyParameters `json:"reply_parameters,omitempty"`
}

// InputFile describes an upload to Telegram.
type InputFile struct {
	// Field is the multipart field name, defaults to the attachment type.
	Field string
	// Filename is the name reported to Telegram.
	Filename string
	// ContentType is the MIME type of the payload.
	ContentType string
	// Data holds the payload bytes.
	Data []byte
	// Path points at a file on disk that should be streamed instead.
	Path string
}

// InputMedia is a single item of a media group.
type InputMedia struct {
	Type              string
	Media             string
	Thumbnail         string
	Caption           string
	ParseMode         string
	CaptionEntities   []MessageEntity
	Width             int
	Height            int
	Duration          int
	SupportsStreaming bool
}

// InputMediaPhoto builds a photo item for a media group.
func InputMediaPhoto(media string, caption string, parseMode string) InputMedia {
	return InputMedia{
		Type:      "photo",
		Media:     media,
		Caption:   caption,
		ParseMode: parseMode,
	}
}

// InputMediaVideo builds a video item for a media group.
func InputMediaVideo(media string, caption string, parseMode string, width, height, duration int, supportsStreaming bool) InputMedia {
	return InputMedia{
		Type:              "video",
		Media:             media,
		Caption:           caption,
		ParseMode:         parseMode,
		Width:             width,
		Height:            height,
		Duration:          duration,
		SupportsStreaming: supportsStreaming,
	}
}

// InputMediaDocument builds a document item for a media group.
func InputMediaDocument(media string, caption string, parseMode string) InputMedia {
	return InputMedia{
		Type:      "document",
		Media:     media,
		Caption:   caption,
		ParseMode: parseMode,
	}
}

// InlineQueryResult is one entry of an inline answer.
type InlineQueryResult struct {
	Type                string                   `json:"type"`
	ID                  string                   `json:"id"`
	Title               string                   `json:"title,omitempty"`
	Description         string                   `json:"description,omitempty"`
	ThumbURL            string                   `json:"thumb_url,omitempty"`
	ContentURL          string                   `json:"content_url,omitempty"`
	MessageText         string                   `json:"message_text,omitempty"`
	ReplyMarkup         *InlineKeyboardMarkup    `json:"reply_markup,omitempty"`
	InputMessageContent *InputTextMessageContent `json:"input_message_content,omitempty"`
}

// InputTextMessageContent is the payload of an inline message.
type InputTextMessageContent struct {
	MessageText           string `json:"message_text"`
	ParseMode             string `json:"parse_mode,omitempty"`
	DisableWebPagePreview bool   `json:"disable_web_page_preview,omitempty"`
}

// APIError describes a Telegram API failure.
type APIError struct {
	Method      string
	Code        int
	Description string
	Parameters  map[string]any
}

func (e *APIError) Error() string {
	if e.Description == "" {
		return fmt.Sprintf("telegram api %s failed with code %d", e.Method, e.Code)
	}
	return fmt.Sprintf("telegram api %s failed with code %d: %s", e.Method, e.Code, e.Description)
}

// IsRetryable reports whether the error is worth retrying.
func (e *APIError) IsRetryable() bool {
	return e.Code == 429 || e.Code >= 500
}

// IsTooLarge reports whether Telegram refused the upload because the file is
// bigger than the limit of the bot.
func (e *APIError) IsTooLarge() bool {
	description := strings.ToLower(e.Description)
	return strings.Contains(description, "too large") ||
		strings.Contains(description, "request_entity_too_large") ||
		strings.Contains(description, "file is too big")
}

// IsNotModified reports whether Telegram refused the edit because the new
// content is identical to the current one.
func (e *APIError) IsNotModified() bool {
	description := strings.ToLower(e.Description)
	return strings.Contains(description, "message is not modified")
}

// IsStaleReference reports whether the cached file identifier belongs to the
// Bot API and cannot be used over MTProto.
func (e *APIError) IsStaleReference() bool {
	return strings.Contains(e.Description, "not a mtproto reference")
}

// RetryAfter returns the retry delay advertised by Telegram.
func (e *APIError) RetryAfter() time.Duration {
	if e.Parameters == nil {
		return 0
	}
	if value, ok := e.Parameters["retry_after"].(float64); ok {
		return time.Duration(value) * time.Second
	}
	return 0
}
