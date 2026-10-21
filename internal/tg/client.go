// Package tg is a small Telegram client that speaks MTProto.
//
// The client replaces the Bot API, a single upload can carry up to 2000 MB
// and the bot needs no local Bot API server.
package tg

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	gogram "github.com/amarnathcjd/gogram/telegram"
)

// Client talks to Telegram over MTProto.
type Client struct {
	raw *gogram.Client

	mu        sync.Mutex
	callbacks map[string]*gogram.CallbackQuery
	inlines   map[string]*gogram.InlineQuery
	seen      map[string]time.Time
	stopped   bool
}

// Option customises a client.
type Option func(*options)

// options holds the settings of a client.
type options struct {
	session  string
	logLevel gogram.LogLevel
}

// WithSession stores the MTProto session in the given file, the session keeps
// the authorization of the bot between two runs.
func WithSession(path string) Option {
	return func(o *options) {
		if strings.TrimSpace(path) != "" {
			o.session = strings.TrimSpace(path)
		}
	}
}

// WithLogLevel sets the verbosity of the Telegram client.
func WithLogLevel(level gogram.LogLevel) Option {
	return func(o *options) { o.logLevel = level }
}

// New builds a client that talks to Telegram over MTProto.
//
// The bot token belongs to the account, the API identifier and the API hash
// come from the Telegram application the bot belongs to.
func New(token string, apiID int32, apiHash string, opts ...Option) (*Client, error) {
	settings := options{logLevel: gogram.LogWarn}
	for _, opt := range opts {
		opt(&settings)
	}

	if settings.session != "" {
		if err := ensureSessionFolder(settings.session); err != nil {
			return nil, err
		}
	}

	config := gogram.ClientConfig{
		AppID:            apiID,
		AppHash:          apiHash,
		ParseMode:        "HTML",
		LogLevel:         settings.logLevel,
		Session:          settings.session,
		MemorySession:    settings.session == "",
		CommandPrefixes:  "/",
		SleepThresholdMs: 1000,
	}

	raw, err := gogram.NewClient(config)
	if err != nil {
		return nil, fmt.Errorf("build the telegram client: %w", err)
	}

	if err := raw.LoginBot(token); err != nil {
		return nil, fmt.Errorf("login the bot over mtproto: %w", err)
	}

	return &Client{
		raw:       raw,
		callbacks: map[string]*gogram.CallbackQuery{},
		inlines:   map[string]*gogram.InlineQuery{},
		seen:      map[string]time.Time{},
	}, nil
}

// Run delivers the incoming updates to the handler until the context is
// cancelled.
func (c *Client) Run(ctx context.Context, handler func(Update)) error {
	c.raw.On(gogram.OnMessage, func(message *gogram.NewMessage) error {
		converted := messageFrom(message)
		if converted == nil {
			return nil
		}
		handler(Update{Message: converted})
		return nil
	})

	c.raw.On(gogram.OnCallback, func(callback *gogram.CallbackQuery) error {
		converted := c.callbackFrom(callback)
		if converted == nil {
			return nil
		}
		handler(Update{CallbackQuery: converted})
		return nil
	})

	c.raw.On(gogram.OnInlineQuery, func(query *gogram.InlineQuery) error {
		converted := c.inlineFrom(query)
		if converted == nil {
			return nil
		}
		handler(Update{InlineQuery: converted})
		return nil
	})

	<-ctx.Done()
	c.Close()
	return nil
}

// Close stops the client and disconnects from Telegram.
func (c *Client) Close() {
	c.mu.Lock()
	if c.stopped {
		c.mu.Unlock()
		return
	}
	c.stopped = true
	c.mu.Unlock()

	c.raw.Stop()
}

// GetMe returns the account of the bot.
func (c *Client) GetMe(ctx context.Context) (*User, error) {
	me, err := c.raw.GetMe()
	if err != nil {
		return nil, errorFrom(err)
	}
	return userFrom(me), nil
}

// SetMyCommands publishes the command menu of the bot.
func (c *Client) SetMyCommands(ctx context.Context, commands []BotCommand) error {
	if len(commands) == 0 {
		return nil
	}
	published := make([]*gogram.BotCommand, 0, len(commands))
	for _, command := range commands {
		published = append(published, &gogram.BotCommand{Command: command.Command, Description: command.Description})
	}
	if _, err := c.raw.BotsSetBotCommands(&gogram.BotCommandScopeDefault{}, "", published); err != nil {
		return errorFrom(err)
	}
	return nil
}

// SendMessageOptions holds the options of a text message.
type SendMessageOptions struct {
	ParseMode           string
	Entities            []MessageEntity
	ReplyMarkup         *InlineKeyboardMarkup
	ReplyKeyboard       *ReplyKeyboardMarkup
	RemoveKeyboard      bool
	DisableNotification bool
	ProtectContent      bool
	MessageThreadID     int64
	ReplyToMessageID    int64
}

// SendMessage sends a text message.
func (c *Client) SendMessage(ctx context.Context, chatID int64, text string, opts SendMessageOptions) (*Message, error) {
	options := &gogram.SendOptions{
		ParseMode:   resolveParseMode(opts.ParseMode),
		Silent:      opts.DisableNotification,
		NoForwards:  opts.ProtectContent,
		ReplyID:     int32(opts.ReplyToMessageID),
		TopicID:     int32(opts.MessageThreadID),
		ReplyMarkup: markupFrom(opts.ReplyMarkup),
	}
	if opts.RemoveKeyboard {
		options.ReplyMarkup = gogram.Button.Clear()
	} else if opts.ReplyKeyboard != nil {
		options.ReplyMarkup = replyMarkupFrom(opts.ReplyKeyboard)
	}
	if len(opts.Entities) > 0 {
		entities, err := entitiesTo(opts.Entities)
		if err == nil {
			options.Entities = entities
		}
	}

	sent, err := c.raw.SendMessage(chatID, text, options)
	if err != nil {
		return nil, errorFrom(err)
	}
	return messageFrom(sent), nil
}

// EditMessageText replaces the text of a message.
func (c *Client) EditMessageText(ctx context.Context, chatID int64, messageID int64, text string, parseMode string, markup *InlineKeyboardMarkup) (*Message, error) {
	edited, err := c.raw.EditMessage(chatID, int32(messageID), text, &gogram.SendOptions{
		ParseMode:   resolveParseMode(parseMode),
		ReplyMarkup: markupFrom(markup),
	})
	if err != nil {
		return nil, errorFrom(err)
	}
	return messageFrom(edited), nil
}

// EditMessageCaption replaces the caption of a message.
func (c *Client) EditMessageCaption(ctx context.Context, chatID int64, messageID int64, caption string, parseMode string, markup *InlineKeyboardMarkup) (*Message, error) {
	edited, err := c.raw.EditMessage(chatID, int32(messageID), "", &gogram.SendOptions{
		ParseMode:   resolveParseMode(parseMode),
		ReplyMarkup: markupFrom(markup),
		Media:       &gogram.InputMediaEmpty{},
	})
	if err != nil {
		return nil, errorFrom(err)
	}
	return messageFrom(edited), nil
}

// SendChatAction shows an action of the bot inside the chat.
func (c *Client) SendChatAction(ctx context.Context, chatID int64, action string) error {
	converted, ok := chatAction(action)
	if !ok {
		return nil
	}
	if _, err := c.raw.SendAction(chatID, converted); err != nil {
		return errorFrom(err)
	}
	return nil
}

// AnswerCallbackQuery answers the press of an inline keyboard button.
func (c *Client) AnswerCallbackQuery(ctx context.Context, callbackID string, text string, showAlert bool) error {
	callback := c.callback(callbackID)
	if callback == nil {
		return nil
	}
	if _, err := callback.Answer(text, &gogram.CallbackOptions{Alert: showAlert}); err != nil {
		return errorFrom(err)
	}
	c.forget(callbackID)
	return nil
}

// AnswerInlineQuery answers an inline query.
func (c *Client) AnswerInlineQuery(ctx context.Context, inlineQueryID string, results []InlineQueryResult, cacheTime int, isPersonal bool, nextOffset string) error {
	query := c.inline(inlineQueryID)
	if query == nil {
		return nil
	}
	options := &gogram.InlineSendOptions{
		CacheTime:  int32(cacheTime),
		Private:    isPersonal,
		NextOffset: nextOffset,
	}
	if _, err := query.Answer(inlineResultsFrom(results), options); err != nil {
		return errorFrom(err)
	}
	c.forget(inlineQueryID)
	return nil
}

// SendPhoto sends a photo that is available on a public address.
func (c *Client) SendPhoto(ctx context.Context, chatID int64, media string, caption string, parseMode string, markup *InlineKeyboardMarkup, hasSpoiler bool) (*Message, error) {
	sent, err := c.raw.SendMedia(chatID, media, &gogram.MediaOptions{
		Caption:     caption,
		ParseMode:   resolveParseMode(parseMode),
		ReplyMarkup: markupFrom(markup),
		Spoiler:     hasSpoiler,
	})
	if err != nil {
		return nil, errorFrom(err)
	}
	return messageFrom(sent), nil
}

// SendPhotoFile sends a photo that is stored on the disk.
func (c *Client) SendPhotoFile(ctx context.Context, chatID int64, path string, caption string, parseMode string, markup *InlineKeyboardMarkup) (*Message, error) {
	sent, err := c.raw.SendMedia(chatID, path, &gogram.MediaOptions{
		Caption:     caption,
		ParseMode:   resolveParseMode(parseMode),
		ReplyMarkup: markupFrom(markup),
		FileName:    fileNameOfPath(path),
	})
	if err != nil {
		return nil, errorFrom(err)
	}
	return messageFrom(sent), nil
}

// SendPhotoBytes sends a photo that is held in the memory.
func (c *Client) SendPhotoBytes(ctx context.Context, chatID int64, data []byte, filename, caption, parseMode string, markup *InlineKeyboardMarkup) (*Message, error) {
	file, err := c.raw.UploadFile(data)
	if err != nil {
		return nil, errorFrom(err)
	}
	sent, err := c.raw.SendMedia(chatID, file, &gogram.MediaOptions{
		Caption:     caption,
		ParseMode:   resolveParseMode(parseMode),
		ReplyMarkup: markupFrom(markup),
		FileName:    filename,
	})
	if err != nil {
		return nil, errorFrom(err)
	}
	return messageFrom(sent), nil
}

// EditMessageMediaPhoto replaces the photo of a message.
func (c *Client) EditMessageMediaPhoto(ctx context.Context, chatID int64, messageID int64, data []byte, filename, caption, parseMode string, markup *InlineKeyboardMarkup) (*Message, error) {
	file, err := c.raw.UploadFile(data)
	if err != nil {
		return nil, errorFrom(err)
	}
	edited, err := c.raw.EditMessage(chatID, int32(messageID), "", &gogram.SendOptions{
		ParseMode:   resolveParseMode(parseMode),
		ReplyMarkup: markupFrom(markup),
		Media:       file,
		FileName:    filename,
	})
	if err != nil {
		return nil, errorFrom(err)
	}
	return messageFrom(edited), nil
}

// SendVideo sends a video that is available on a public address.
func (c *Client) SendVideo(ctx context.Context, chatID int64, media string, caption string, parseMode string, duration int, width, height int, supportsStreaming bool, markup *InlineKeyboardMarkup) (*Message, error) {
	sent, err := c.raw.SendMedia(chatID, media, &gogram.MediaOptions{
		Caption:     caption,
		ParseMode:   resolveParseMode(parseMode),
		ReplyMarkup: markupFrom(markup),
		Attributes:  videoAttributes(duration, width, height, supportsStreaming),
	})
	if err != nil {
		return nil, errorFrom(err)
	}
	return messageFrom(sent), nil
}

// SendVideoFile sends a video that is stored on the disk.
func (c *Client) SendVideoFile(ctx context.Context, chatID int64, path string, thumbPath string, caption string, parseMode string, duration int, width, height int, supportsStreaming bool, markup *InlineKeyboardMarkup) (*Message, error) {
	options := &gogram.MediaOptions{
		Caption:     caption,
		ParseMode:   resolveParseMode(parseMode),
		ReplyMarkup: markupFrom(markup),
		Attributes:  videoAttributes(duration, width, height, supportsStreaming),
		FileName:    fileNameOfPath(path),
	}
	if thumbPath != "" {
		options.Thumb = thumbPath
	}
	sent, err := c.raw.SendMedia(chatID, path, options)
	if err != nil {
		return nil, errorFrom(err)
	}
	return messageFrom(sent), nil
}

// SendDocument sends a document that is available on a public address.
func (c *Client) SendDocument(ctx context.Context, chatID int64, media string, caption string, parseMode string, disableContentTypeDetection bool, markup *InlineKeyboardMarkup) (*Message, error) {
	sent, err := c.raw.SendMedia(chatID, media, &gogram.MediaOptions{
		Caption:       caption,
		ParseMode:     resolveParseMode(parseMode),
		ReplyMarkup:   markupFrom(markup),
		ForceDocument: true,
	})
	if err != nil {
		return nil, errorFrom(err)
	}
	return messageFrom(sent), nil
}

// SendDocumentFile sends a document that is stored on the disk.
func (c *Client) SendDocumentFile(ctx context.Context, chatID int64, path string, thumbPath string, caption string, parseMode string, disableContentTypeDetection bool, markup *InlineKeyboardMarkup) (*Message, error) {
	options := &gogram.MediaOptions{
		Caption:       caption,
		ParseMode:     resolveParseMode(parseMode),
		ReplyMarkup:   markupFrom(markup),
		ForceDocument: true,
		FileName:      fileNameOfPath(path),
	}
	if thumbPath != "" {
		options.Thumb = thumbPath
	}
	sent, err := c.raw.SendMedia(chatID, path, options)
	if err != nil {
		return nil, errorFrom(err)
	}
	return messageFrom(sent), nil
}

// SendAudio sends an audio file that is available on a public address.
func (c *Client) SendAudio(ctx context.Context, chatID int64, media string, caption string, parseMode string, duration int, performer, title string, markup *InlineKeyboardMarkup) (*Message, error) {
	sent, err := c.raw.SendMedia(chatID, media, &gogram.MediaOptions{
		Caption:     caption,
		ParseMode:   resolveParseMode(parseMode),
		ReplyMarkup: markupFrom(markup),
		Attributes:  audioAttributes(duration, performer, title),
	})
	if err != nil {
		return nil, errorFrom(err)
	}
	return messageFrom(sent), nil
}

// SendAudioFile sends an audio file that is stored on the disk.
func (c *Client) SendAudioFile(ctx context.Context, chatID int64, path string, thumbPath string, caption string, parseMode string, duration int, performer, title string, markup *InlineKeyboardMarkup) (*Message, error) {
	options := &gogram.MediaOptions{
		Caption:     caption,
		ParseMode:   resolveParseMode(parseMode),
		ReplyMarkup: markupFrom(markup),
		Attributes:  audioAttributes(duration, performer, title),
		FileName:    fileNameOfPath(path),
	}
	if thumbPath != "" {
		options.Thumb = thumbPath
	}
	sent, err := c.raw.SendMedia(chatID, path, options)
	if err != nil {
		return nil, errorFrom(err)
	}
	return messageFrom(sent), nil
}

// SendMediaGroup sends a group of media files.
func (c *Client) SendMediaGroup(ctx context.Context, chatID int64, media []InputMedia) (*Message, error) {
	if len(media) == 0 {
		return nil, nil
	}
	album := make([]any, 0, len(media))
	for _, item := range media {
		switch item.Type {
		case "photo":
			album = append(album, &gogram.InputMediaPhoto{})
		default:
			album = append(album, &gogram.InputMediaDocument{})
		}
	}
	sent, err := c.raw.SendAlbum(chatID, album, &gogram.MediaOptions{})
	if err != nil {
		return nil, errorFrom(err)
	}
	if len(sent) == 0 {
		return nil, nil
	}
	return messageFrom(sent[0]), nil
}

// SendCachedFile sends a file that already lives on the Telegram servers.
//
// The reference is the value that the cache of the bot stores, the file is
// copied to the chat without a new upload.
func (c *Client) SendCachedFile(ctx context.Context, chatID int64, reference string, caption string, markup *InlineKeyboardMarkup) (*Message, error) {
	parsed, ok := decodeReference(reference)
	if !ok {
		return nil, &APIError{Method: "sendCachedFile", Description: "the cached file is not a mtproto reference"}
	}
	sent, err := c.raw.SendMedia(chatID, parsed.document(), &gogram.MediaOptions{
		Caption:     caption,
		ParseMode:   "HTML",
		ReplyMarkup: markupFrom(markup),
	})
	if err != nil {
		return nil, errorFrom(err)
	}
	return messageFrom(sent), nil
}

// GetChat returns the chat with the given identifier.
func (c *Client) GetChat(ctx context.Context, chatID int64) (*Chat, error) {
	peer, err := c.raw.ResolvePeer(chatID)
	if err != nil {
		return nil, errorFrom(err)
	}
	switch typed := peer.(type) {
	case *gogram.InputPeerUser:
		return &Chat{ID: typed.UserID, Type: "private"}, nil
	case *gogram.InputPeerChat:
		return &Chat{ID: -typed.ChatID, Type: "group"}, nil
	case *gogram.InputPeerChannel:
		return &Chat{ID: channelChatID(typed.ChannelID), Type: "supergroup"}, nil
	default:
		return &Chat{ID: chatID, Type: "private"}, nil
	}
}

// GetChatMember returns the membership of a user inside a chat.
func (c *Client) GetChatMember(ctx context.Context, chatID, userID int64) (*ChatMember, error) {
	participant, err := c.raw.GetChatMember(chatID, userID)
	if err == nil {
		return chatMemberOf(participant), nil
	}

	// A basic group knows no single member call, the full chat of the group
	// holds the members of the group.
	member, fallbackErr := c.memberOfBasicGroup(chatID, userID)
	if fallbackErr == nil {
		return member, nil
	}
	return nil, errorFrom(err)
}

// memberOfBasicGroup reads a member out of the full chat of a basic group.
func (c *Client) memberOfBasicGroup(chatID, userID int64) (*ChatMember, error) {
	full, err := c.raw.MessagesGetFullChat(chatID)
	if err != nil {
		return nil, err
	}
	chat, ok := full.FullChat.(*gogram.ChatFullObj)
	if !ok {
		return nil, &APIError{Method: "getChatMember", Description: "the chat is not a basic group"}
	}
	participants, ok := chat.Participants.(*gogram.ChatParticipantsObj)
	if !ok {
		return nil, &APIError{Method: "getChatMember", Description: "the members of the group are not known"}
	}
	for _, participant := range participants.Participants {
		switch typed := participant.(type) {
		case *gogram.ChatParticipantCreator:
			if typed.UserID == userID {
				return &ChatMember{User: &User{ID: typed.UserID}, Status: "creator"}, nil
			}
		case *gogram.ChatParticipantAdmin:
			if typed.UserID == userID {
				return &ChatMember{User: &User{ID: typed.UserID}, Status: "administrator"}, nil
			}
		case *gogram.ChatParticipantObj:
			if typed.UserID == userID {
				return &ChatMember{User: &User{ID: typed.UserID}, Status: "member"}, nil
			}
		}
	}
	return nil, &APIError{Method: "getChatMember", Description: "the user is not a member of the chat"}
}

// chatMemberOf converts a participant of a supergroup.
func chatMemberOf(participant *gogram.Participant) *ChatMember {
	if participant == nil {
		return nil
	}
	return &ChatMember{
		User:   userFrom(participant.User),
		Status: memberStatus(participant.Status),
	}
}

// memberStatus translates the status of a participant into the names of the
// Bot API.
func memberStatus(status string) string {
	switch status {
	case gogram.Admin:
		return "administrator"
	case gogram.Creator:
		return "creator"
	case gogram.Member:
		return "member"
	case gogram.Restricted:
		return "restricted"
	case gogram.Left:
		return "left"
	case gogram.Kicked:
		return "kicked"
	default:
		return status
	}
}

// DeleteMessage removes a message from the chat.
func (c *Client) DeleteMessage(ctx context.Context, chatID, messageID int64) error {
	if _, err := c.raw.DeleteMessages(chatID, []int32{int32(messageID)}); err != nil {
		return errorFrom(err)
	}
	return nil
}

// callbackFrom converts an incoming callback query.
func (c *Client) callbackFrom(callback *gogram.CallbackQuery) *CallbackQuery {
	if callback == nil {
		return nil
	}
	converted := &CallbackQuery{
		ID:   strconv.FormatInt(callback.QueryID, 10),
		From: userFrom(callback.Sender),
		Data: callback.DataString(),
	}
	if message, err := callback.GetMessage(); err == nil && message != nil {
		converted.Message = messageFrom(message)
	}
	c.remember(converted.ID, callback)
	return converted
}

// inlineFrom converts an incoming inline query.
func (c *Client) inlineFrom(query *gogram.InlineQuery) *InlineQuery {
	if query == nil {
		return nil
	}
	converted := &InlineQuery{
		ID:     strconv.FormatInt(query.QueryID, 10),
		From:   userFrom(query.Sender),
		Query:  query.Query,
		Offset: query.Offset,
	}
	switch query.PeerType {
	case gogram.InlineQueryPeerTypeChat, gogram.InlineQueryPeerTypeMegagroup, gogram.InlineQueryPeerTypeBroadcast:
		converted.ChatType = "group"
	default:
		converted.ChatType = "private"
	}
	c.remember(converted.ID, query)
	return converted
}

// remember keeps the raw object of an update, the answer of a callback and of
// an inline query needs it.
func (c *Client) remember(id string, value any) {
	c.mu.Lock()
	defer c.mu.Unlock()
	now := time.Now()
	for key, seen := range c.seen {
		if now.Sub(seen) > 10*time.Minute {
			delete(c.seen, key)
			delete(c.callbacks, key)
			delete(c.inlines, key)
		}
	}
	switch typed := value.(type) {
	case *gogram.CallbackQuery:
		c.callbacks[id] = typed
	case *gogram.InlineQuery:
		c.inlines[id] = typed
	}
	c.seen[id] = now
}

// callback returns the raw object of a callback query.
func (c *Client) callback(id string) *gogram.CallbackQuery {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.callbacks[id]
}

// inline returns the raw object of an inline query.
func (c *Client) inline(id string) *gogram.InlineQuery {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.inlines[id]
}

// forget drops the raw object of an update.
func (c *Client) forget(id string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.callbacks, id)
	delete(c.inlines, id)
	delete(c.seen, id)
}

// videoAttributes builds the attributes of a video file.
func videoAttributes(duration int, width, height int, supportsStreaming bool) []gogram.DocumentAttribute {
	return []gogram.DocumentAttribute{
		&gogram.DocumentAttributeVideo{
			Duration:          float64(duration),
			W:                 int32(width),
			H:                 int32(height),
			SupportsStreaming: supportsStreaming,
		},
	}
}

// audioAttributes builds the attributes of an audio file.
func audioAttributes(duration int, performer, title string) []gogram.DocumentAttribute {
	return []gogram.DocumentAttribute{
		&gogram.DocumentAttributeAudio{
			Duration:  int32(duration),
			Performer: performer,
			Title:     title,
		},
	}
}

// entitiesTo converts the formatting entities of the package into the
// entities of the Telegram client.
func entitiesTo(entities []MessageEntity) ([]gogram.MessageEntity, error) {
	converted := make([]gogram.MessageEntity, 0, len(entities))
	for _, entity := range entities {
		switch entity.Type {
		case "bold":
			converted = append(converted, &gogram.MessageEntityBold{Offset: int32(entity.Offset), Length: int32(entity.Length)})
		case "italic":
			converted = append(converted, &gogram.MessageEntityItalic{Offset: int32(entity.Offset), Length: int32(entity.Length)})
		case "code":
			converted = append(converted, &gogram.MessageEntityCode{Offset: int32(entity.Offset), Length: int32(entity.Length)})
		case "pre":
			converted = append(converted, &gogram.MessageEntityPre{Offset: int32(entity.Offset), Length: int32(entity.Length), Language: entity.Language})
		case "text_link":
			converted = append(converted, &gogram.MessageEntityTextURL{Offset: int32(entity.Offset), Length: int32(entity.Length), URL: entity.URL})
		default:
			return nil, fmt.Errorf("the entity %q is not supported", entity.Type)
		}
	}
	return converted, nil
}

// chatAction converts the action of the Bot API into the name of the action
// of the Telegram client.
func chatAction(action string) (string, bool) {
	name := strings.ToLower(strings.TrimSpace(action))
	switch name {
	case "record_voice":
		name = "record_audio"
	}
	_, ok := gogram.Actions[name]
	return name, ok
}

// resolveParseMode returns the parse mode of the Telegram client.
func resolveParseMode(mode string) string {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case "":
		return ""
	case "html":
		return "html"
	case "markdown":
		return "markdown"
	case "markdownv2":
		return "markdown"
	default:
		return "html"
	}
}

// fileNameOfPath returns the name of a file without the folders.
func fileNameOfPath(path string) string {
	if path == "" {
		return ""
	}
	index := strings.LastIndexAny(path, "/\\")
	if index < 0 {
		return path
	}
	return path[index+1:]
}

// ensureSessionFolder creates the folder that holds the session file.
func ensureSessionFolder(path string) error {
	folder := filepath.Dir(path)
	if folder == "" || folder == "." {
		return nil
	}
	if err := os.MkdirAll(folder, 0o750); err != nil {
		return fmt.Errorf("create the session folder %q: %w", folder, err)
	}
	return nil
}
