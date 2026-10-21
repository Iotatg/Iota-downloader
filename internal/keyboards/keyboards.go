// Package keyboards builds every Telegram keyboard used by the Iota Coder bot.
package keyboards

import (
	"fmt"
	"math/rand/v2"
	"strings"

	"github.com/Iotatg/iota-downloader/internal/messages"
	"github.com/Iotatg/iota-downloader/internal/settings"
	"github.com/Iotatg/iota-downloader/internal/text"
	"github.com/Iotatg/iota-downloader/internal/tg"
)

// Callback prefixes used across the bot.
const (
	PrefixSettings    = "settings:"
	PrefixSetting     = "setting:"
	PrefixSettingsCat = "settings_cat:"
	PrefixAdmin       = "admin_"
	PrefixMailing     = "mailing_target:"
	PrefixStats       = "stats:"
	PrefixInline      = "inline:"
	PrefixAudio       = "audio:"
	PrefixDocument    = "doc:"
)

// Callback data values of the admin panel.
const (
	AdminOps              = "admin_ops"
	AdminRefresh          = "admin_refresh"
	AdminRuntimeStorage   = "admin_runtime_storage"
	AdminDownloadHistory  = "admin_download_history"
	AdminCleanupDownloads = "admin_cleanup_downloads"
	AdminCheckUsers       = "check_active_users"
	AdminCheckGroups      = "check_active_groups"
	AdminMailing          = "admin_mailing"
	AdminBack             = "back_to_admin"
	AdminNoop             = "noop"
	AdminDeleteLog        = "delete_log"
	AdminDownloadLog      = "download_log"
	AdminCancel           = "cancel_action"
	AdminSendToAll        = "send_to_all"
)

// Button colours, named the way the Bot API presents them.
const (
	ColourRed   = "red"
	ColourBlue  = "blue"
	ColourGreen = "green"
)

// buttonColours holds every colour a button can wear.
var buttonColours = []string{ColourRed, ColourBlue, ColourGreen}

// colourEnabled turns the button colours on or off.
var colourEnabled = true

// SetColourEnabled turns the random button colours on or off.
func SetColourEnabled(enabled bool) { colourEnabled = enabled }

// RandomColour returns a random button colour.
func RandomColour() string {
	return buttonColours[rand.IntN(len(buttonColours))]
}

// StyleFor maps a colour onto the button style of the Bot API.
func StyleFor(colour string) string {
	switch strings.ToLower(colour) {
	case ColourRed:
		return tg.ButtonStyleDanger
	case ColourBlue:
		return tg.ButtonStylePrimary
	case ColourGreen:
		return tg.ButtonStyleSuccess
	default:
		return ""
	}
}

// randomStyle returns the style of a random button colour.
//
// An empty style lets the client fall back to its own default colour.
func randomStyle() string {
	if !colourEnabled {
		return ""
	}
	return StyleFor(RandomColour())
}

// MaxCallbackData is the number of bytes a callback data may hold.
//
// The Bot API rejects a longer value, so a callback that carries a long url
// is shortened to a button that does nothing.
const MaxCallbackData = 64

// safeCallbackData keeps a callback data inside the limit of the Bot API.
func safeCallbackData(callbackData string) string {
	if callbackData == "" || len(callbackData) > MaxCallbackData {
		return AdminNoop
	}
	return callbackData
}

// Button builds a callback button with a random colour.
func Button(label, callbackData string) tg.InlineKeyboardButton {
	return tg.InlineKeyboardButton{Text: label, CallbackData: safeCallbackData(callbackData), Style: randomStyle()}
}

// StyledButton builds a callback button with an explicit colour.
func StyledButton(label, callbackData, style string) tg.InlineKeyboardButton {
	return tg.InlineKeyboardButton{Text: label, CallbackData: safeCallbackData(callbackData), Style: style}
}

// ColorButton builds a callback button whose label starts with a colourful
// glyph. Beside the glyph the button wears a random colour, so every button of
// a keyboard looks different from its neighbours.
func ColorButton(label, callbackData string) tg.InlineKeyboardButton {
	return tg.InlineKeyboardButton{
		Text:         text.AccentLabel(callbackData, label),
		CallbackData: safeCallbackData(callbackData),
		Style:        randomStyle(),
	}
}

// ColorStyledButton builds a callback button whose label starts with a
// colourful glyph and wears an explicit colour.
func ColorStyledButton(label, callbackData, style string) tg.InlineKeyboardButton {
	return tg.InlineKeyboardButton{
		Text:         text.AccentLabel(callbackData, label),
		CallbackData: safeCallbackData(callbackData),
		Style:        style,
	}
}

// URLButton builds a link button with a random colour.
func URLButton(label, url string) tg.InlineKeyboardButton {
	return tg.InlineKeyboardButton{Text: label, URL: url, Style: randomStyle()}
}

// Row is a helper for a single row of buttons.
func Row(buttons ...tg.InlineKeyboardButton) []tg.InlineKeyboardButton { return buttons }

// Markup wraps rows into an inline keyboard.
func Markup(rows ...[]tg.InlineKeyboardButton) *tg.InlineKeyboardMarkup {
	return &tg.InlineKeyboardMarkup{InlineKeyboard: rows}
}

// SettingsKeyboard builds the root of the settings menu.
func SettingsKeyboard() *tg.InlineKeyboardMarkup {
	return Markup(
		Row(
			ColorButton(text.Label(text.SymLeft, "media & quality"), PrefixSettingsCat+settings.CategoryMedia),
			ColorButton(text.Label(text.SymRight, "appearance"), PrefixSettingsCat+settings.CategoryAppearance),
		),
		Row(
			ColorButton(text.Label(text.SymUp, "chat & clean up"), PrefixSettingsCat+settings.CategoryChat),
			ColorStyledButton(text.Label(text.SymErase, "close"), AdminNoop, tg.ButtonStyleDanger),
		),
	)
}

// CategoryKeyboard builds the keyboard of a settings category.
//
// Every button carries the current value of the option, so the user sees the
// active choice without opening every option.
func CategoryKeyboard(category string, set settings.Set) *tg.InlineKeyboardMarkup {
	rows := make([][]tg.InlineKeyboardButton, 0, 5)
	for _, field := range settings.Fields {
		if settings.FieldCategory[field] != category {
			continue
		}
		rows = append(rows, Row(ColorButton(fieldLabel(field, set), PrefixSettings+field)))
	}
	rows = append(rows, Row(ColorButton(text.Label(text.SymBack, "back"), "back_to_settings")))
	return Markup(rows...)
}

// fieldLabel renders the button label of a setting with its current value.
func fieldLabel(field string, set settings.Set) string {
	name := fieldName(field)
	value := settings.ValueLabel(field, set.Get(field))
	if value == "" {
		return text.Label(text.SymBullet, name)
	}
	return text.Label(text.SymBullet, name) + " " + text.Small("· "+value)
}

// fieldName renders the name of a setting.
func fieldName(field string) string {
	switch field {
	case settings.FieldVideoQuality:
		return "video quality"
	case settings.FieldAsDocument:
		return "send as file"
	case settings.FieldAudioFormat:
		return "audio format"
	case settings.FieldCaptions:
		return "descriptions"
	case settings.FieldInfoButtons:
		return "info buttons"
	case settings.FieldAudioButton:
		return "mp3 button"
	case settings.FieldFileButton:
		return "file button"
	case settings.FieldURLButton:
		return "url button"
	case settings.FieldDeleteMessage:
		return "delete messages"
	default:
		return field
	}
}

// mark renders the label of an entry of a selector, the active entry wears
// the filled star.
func mark(active bool, value string) string {
	if active {
		return text.SymStar + " " + text.Small(value)
	}
	return text.SymStarOut + " " + text.Small(value)
}

// ToggleKeyboard builds the on/off selector of a boolean setting.
func ToggleKeyboard(field, current string) *tg.InlineKeyboardMarkup {
	on, off := settings.ToggleLabels(field)
	return Markup(
		Row(
			ColorButton(mark(current == settings.On, on), PrefixSetting+field+":"+settings.On),
			ColorButton(mark(current == settings.Off, off), PrefixSetting+field+":"+settings.Off),
		),
		Row(ColorButton(text.Label(text.SymBack, "back"), "back_to_settings")),
	)
}

// VideoQualityKeyboard builds the quality selector.
//
// The buttons are ordered from the highest resolution to the lowest one and
// wear the label of the preference, the active one carries the filled star.
func VideoQualityKeyboard(current string) *tg.InlineKeyboardMarkup {
	qualities := settings.VideoQualities()
	rows := make([][]tg.InlineKeyboardButton, 0, len(qualities)/2+1)
	for index := 0; index < len(qualities); index += 2 {
		row := []tg.InlineKeyboardButton{}
		for _, quality := range qualities[index:min(index+2, len(qualities))] {
			row = append(row, ColorButton(mark(current == quality, settings.VideoQualityLabel(quality)),
				PrefixSetting+settings.FieldVideoQuality+":"+quality))
		}
		rows = append(rows, row)
	}
	rows = append(rows, Row(ColorButton(text.Label(text.SymBack, "back"), "back_to_settings")))
	return Markup(rows...)
}

// AudioFormatKeyboard builds the audio format selector.
func AudioFormatKeyboard(current string) *tg.InlineKeyboardMarkup {
	return Markup(
		Row(
			ColorButton(mark(current == settings.MP3, settings.AudioFormatLabel(settings.MP3)), PrefixSetting+settings.FieldAudioFormat+":"+settings.MP3),
			ColorButton(mark(current == settings.M4A, settings.AudioFormatLabel(settings.M4A)), PrefixSetting+settings.FieldAudioFormat+":"+settings.M4A),
		),
		Row(
			ColorButton(mark(current == settings.FLAC, settings.AudioFormatLabel(settings.FLAC)), PrefixSetting+settings.FieldAudioFormat+":"+settings.FLAC),
			ColorButton(text.Label(text.SymBack, "back"), "back_to_settings"),
		),
	)
}

// BackToSettings returns the keyboard shown on a settings detail page.
func BackToSettings() *tg.InlineKeyboardMarkup {
	return Markup(Row(ColorButton(text.Label(text.SymBack, "back"), "back_to_settings")))
}

// AdminKeyboard builds the admin panel.
func AdminKeyboard() *tg.InlineKeyboardMarkup {
	return Markup(
		Row(
			ColorButton(text.Label(text.SymUp, "operations"), AdminOps),
			ColorButton(text.Label(text.SymRight, "refresh"), AdminRefresh),
		),
		Row(
			ColorButton(text.Label(text.SymLeft, "runtime & storage"), AdminRuntimeStorage),
			ColorButton(text.Label(text.SymUpLeft, "download history"), AdminDownloadHistory),
		),
		Row(
			ColorButton(text.Label(text.SymDelete, "cleanup downloads"), AdminCleanupDownloads),
			ColorButton(text.Label(text.SymForward, "mailing"), AdminMailing),
		),
		Row(
			ColorButton(text.Label(text.SymUp, "check active users"), AdminCheckUsers),
			ColorButton(text.Label(text.SymUp, "check active groups"), AdminCheckGroups),
		),
		Row(ColorStyledButton(text.Label(text.SymErase, "close"), AdminNoop, tg.ButtonStyleDanger)),
	)
}

// AdminOpsKeyboard builds the operations submenu of the admin panel.
func AdminOpsKeyboard() *tg.InlineKeyboardMarkup {
	return Markup(
		Row(
			ColorButton(text.Label(text.SymUp, "active users"), AdminCheckUsers),
			ColorButton(text.Label(text.SymUp, "active groups"), AdminCheckGroups),
		),
		Row(
			ColorButton(text.Label(text.SymLeft, "runtime & storage"), AdminRuntimeStorage),
			ColorButton(text.Label(text.SymUpLeft, "download history"), AdminDownloadHistory),
		),
		Row(
			ColorButton(text.Label(text.SymDelete, "cleanup downloads"), AdminCleanupDownloads),
			ColorButton(text.Label(text.SymForward, "mailing"), AdminMailing),
		),
		Row(ColorButton(text.Label(text.SymBack, "back"), AdminBack)),
	)
}

// MailingAudienceKeyboard builds the audience selector of the mailing tool.
func MailingAudienceKeyboard() *tg.InlineKeyboardMarkup {
	return Markup(
		Row(
			ColorButton(text.Label(text.SymUp, "all users"), PrefixMailing+"all"),
			ColorButton(text.Label(text.SymUp, "groups"), PrefixMailing+"groups"),
		),
		Row(
			ColorButton(text.Label(text.SymUp, "private chats"), PrefixMailing+"dm"),
			ColorStyledButton(text.Label(text.SymDelete, "cancel"), AdminCancel, tg.ButtonStyleDanger),
		),
	)
}

// MailingSendKeyboard is shown after the audience was picked.
func MailingSendKeyboard() *tg.InlineKeyboardMarkup {
	return Markup(
		Row(ColorButton(text.Label(text.SymForward, "send to all"), AdminSendToAll)),
		Row(ColorStyledButton(text.Label(text.SymDelete, "cancel"), AdminCancel, tg.ButtonStyleDanger)),
	)
}

// StatsKeyboard builds the period selector of the statistics screen.
func StatsKeyboard(period, mode string) *tg.InlineKeyboardMarkup {
	periods := []string{"Week", "Month", "Year"}
	buttons := make([]tg.InlineKeyboardButton, 0, len(periods))
	for _, candidate := range periods {
		buttons = append(buttons, ColorButton(mark(candidate == period, candidate), fmt.Sprintf("%s%s:%s", PrefixStats, candidate, mode)))
	}

	// The button switches to the other view, the label shows the view the
	// button switches to.
	toggleTarget := "total"
	toggleLabel := text.Label(text.SymRight, "split view: off")
	if mode == "total" {
		toggleTarget = "split"
		toggleLabel = text.Label(text.SymRight, "split view: on")
	}
	toggle := ColorButton(toggleLabel, fmt.Sprintf("%s%s:%s", PrefixStats, period, toggleTarget))

	return Markup(Row(buttons...), Row(toggle))
}

// InlineSendMediaKeyboard builds the single button used by inline results.
func InlineSendMediaKeyboard(label, callbackData string) *tg.InlineKeyboardMarkup {
	return Markup(Row(Button(label, callbackData)))
}

// InlineAlbumKeyboard builds the keyboard of an inline album.
func InlineAlbumKeyboard(albumURL, token string) *tg.InlineKeyboardMarkup {
	return Markup(
		Row(ColorButton(messages.InlineOpenFullAlbumButton(), PrefixInline+token)),
		Row(URLButton(text.Label(text.SymForward, "open in browser"), albumURL)),
	)
}

// MediaInfo describes the counters shown under a delivered media item.
type MediaInfo struct {
	Views    *int64
	Likes    *int64
	Comments *int64
	Shares   *int64
	VideoURL string
}

// VideoInfoKeyboard builds the optional info row of a delivered video.
func VideoInfoKeyboard(info MediaInfo, set settings.Set, audioCallback, fileCallback string) *tg.InlineKeyboardMarkup {
	var rows [][]tg.InlineKeyboardButton

	if set.IsOn(settings.FieldInfoButtons) {
		row := make([]tg.InlineKeyboardButton, 0, 4)
		if info.Views != nil {
			formatted := messages.FormatNumber(*info.Views)
			row = append(row, Button(text.Label(text.SymUpLeft, "views "+formatted), "views_"+formatted))
		}
		if info.Likes != nil {
			formatted := messages.FormatNumber(*info.Likes)
			row = append(row, Button(text.Label(text.SymStar, "likes "+formatted), "likes_"+formatted))
		}
		if info.Comments != nil {
			formatted := messages.FormatNumber(*info.Comments)
			row = append(row, Button(text.Label(text.SymRight, "comments "+formatted), "comments_"+formatted))
		}
		if info.Shares != nil {
			formatted := messages.FormatNumber(*info.Shares)
			row = append(row, Button(text.Label(text.SymForward, "shares "+formatted), "shares_"+formatted))
		}
		if len(row) > 0 {
			rows = append(rows, row)
		}
	}

	if set.IsOn(settings.FieldAudioButton) && audioCallback != "" && len(audioCallback) <= MaxCallbackData {
		rows = append(rows, Row(ColorButton(text.Label(text.SymMusic, "mp3"), audioCallback)))
	}

	if set.IsOn(settings.FieldFileButton) && !set.IsOn(settings.FieldAsDocument) && fileCallback != "" && len(fileCallback) <= MaxCallbackData {
		rows = append(rows, Row(ColorButton(text.Label(text.SymDownload, "file"), fileCallback)))
	}

	if set.IsOn(settings.FieldURLButton) && info.VideoURL != "" {
		rows = append(rows, Row(URLButton(text.Label(text.SymLink, "url"), info.VideoURL)))
	}

	return Markup(rows...)
}

// UserInfoKeyboard builds the author row of a delivered media item.
func UserInfoKeyboard(nickname string, followers, videos, likes *int64, url string) *tg.InlineKeyboardMarkup {
	rows := make([][]tg.InlineKeyboardButton, 0, 2)
	if nickname != "" {
		if url != "" {
			rows = append(rows, Row(URLButton(nickname, url)))
		} else {
			rows = append(rows, Row(Button(nickname, AdminNoop)))
		}
	}

	row := make([]tg.InlineKeyboardButton, 0, 3)
	if followers != nil {
		formatted := messages.FormatNumber(*followers)
		row = append(row, Button(text.Label(text.SymUp, "followers "+formatted), "followers_"+formatted))
	}
	if videos != nil {
		formatted := messages.FormatNumber(*videos)
		row = append(row, Button(text.Label(text.SymUp, "videos "+formatted), "videos_"+formatted))
	}
	if likes != nil {
		formatted := messages.FormatNumber(*likes)
		row = append(row, Button(text.Label(text.SymStar, "likes "+formatted), "likes_"+formatted))
	}
	if len(row) > 0 {
		rows = append(rows, row)
	}

	return Markup(rows...)
}

// StartKeyboard is the reply keyboard shown in private chats.
func StartKeyboard() *tg.ReplyKeyboardMarkup {
	return &tg.ReplyKeyboardMarkup{
		ResizeKeyboard:  true,
		OneTimeKeyboard: true,
		Keyboard: [][]tg.KeyboardButton{
			{
				{Text: text.Label(text.SymDownload, "download"), Style: randomStyle()},
				{Text: text.Label(text.SymInfo, "help"), Style: randomStyle()},
			},
			{
				{Text: text.Label(text.SymMenu, "settings"), Style: randomStyle()},
				{Text: text.Label(text.SymChart, "stats"), Style: randomStyle()},
			},
		},
	}
}

// RemoveKeyboard hides the reply keyboard.
func RemoveKeyboard() *tg.ReplyKeyboardRemove {
	return &tg.ReplyKeyboardRemove{RemoveKeyboard: true}
}
