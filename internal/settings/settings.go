// Package settings defines the per-user preferences of the Iota Coder bot and
// the helpers used by the inline settings menu.
package settings

import (
	"fmt"
	"strings"
)

// Values accepted by the settings menu.
const (
	On       = "on"
	Off      = "off"
	Best     = "best"
	Balanced = "balanced"
	Saver    = "saver"
	Ultra    = "ultra"
	MP3      = "mp3"
	M4A      = "m4a"
	FLAC     = "flac"
)

// Categories of the settings menu.
const (
	CategoryMedia      = "media"
	CategoryAppearance = "appearance"
	CategoryChat       = "chat"
)

// Field names.
const (
	FieldCaptions      = "captions"
	FieldDeleteMessage = "delete_message"
	FieldInfoButtons   = "info_buttons"
	FieldURLButton     = "url_button"
	FieldAudioButton   = "audio_button"
	FieldFileButton    = "file_button"
	FieldVideoQuality  = "video_quality"
	FieldAsDocument    = "as_document"
	FieldAudioFormat   = "audio_format"
)

// Fields lists every configurable option in menu order.
var Fields = []string{
	FieldVideoQuality,
	FieldAsDocument,
	FieldAudioFormat,
	FieldCaptions,
	FieldInfoButtons,
	FieldAudioButton,
	FieldFileButton,
	FieldURLButton,
	FieldDeleteMessage,
}

// ToggleFields lists the boolean options.
var ToggleFields = []string{
	FieldCaptions,
	FieldDeleteMessage,
	FieldInfoButtons,
	FieldURLButton,
	FieldAudioButton,
	FieldFileButton,
	FieldAsDocument,
}

// Values lists every accepted value of a setting.
var Values = map[string]bool{
	On: true, Off: true,
	Best: true, Balanced: true, Saver: true, Ultra: true,
	MP3: true, M4A: true, FLAC: true,
}

// Defaults holds the initial value of every setting.
var Defaults = map[string]string{
	FieldCaptions:      Off,
	FieldDeleteMessage: Off,
	FieldInfoButtons:   Off,
	FieldURLButton:     Off,
	FieldAudioButton:   Off,
	FieldFileButton:    Off,
	FieldVideoQuality:  Best,
	FieldAsDocument:    Off,
	FieldAudioFormat:   MP3,
}

// FieldCategory maps a setting onto the menu category that owns it.
var FieldCategory = map[string]string{
	FieldVideoQuality:  CategoryMedia,
	FieldAsDocument:    CategoryMedia,
	FieldAudioFormat:   CategoryMedia,
	FieldCaptions:      CategoryAppearance,
	FieldInfoButtons:   CategoryAppearance,
	FieldAudioButton:   CategoryAppearance,
	FieldFileButton:    CategoryAppearance,
	FieldURLButton:     CategoryAppearance,
	FieldDeleteMessage: CategoryChat,
}

// Categories lists the menu categories in display order.
var Categories = []string{CategoryMedia, CategoryAppearance, CategoryChat}

// IsKnownField reports whether the field can be configured.
func IsKnownField(field string) bool {
	for _, candidate := range Fields {
		if candidate == field {
			return true
		}
	}
	return false
}

// IsKnownValue reports whether the value is accepted for the given field.
func IsKnownValue(field, value string) bool {
	if !IsKnownField(field) {
		return false
	}
	switch field {
	case FieldVideoQuality:
		return value == Best || value == Balanced || value == Saver || value == Ultra
	case FieldAudioFormat:
		return value == MP3 || value == M4A || value == FLAC
	default:
		return value == On || value == Off
	}
}

// Default returns the default value of a field.
func Default(field string) string {
	if value, ok := Defaults[field]; ok {
		return value
	}
	return Off
}

// Set is the resolved preference set of a single user.
type Set struct {
	Captions      string
	DeleteMessage string
	InfoButtons   string
	URLButton     string
	AudioButton   string
	FileButton    string
	VideoQuality  string
	AsDocument    string
	AudioFormat   string
}

// DefaultSet returns the preference set used for new users.
func DefaultSet() Set {
	return Set{
		Captions:      Defaults[FieldCaptions],
		DeleteMessage: Defaults[FieldDeleteMessage],
		InfoButtons:   Defaults[FieldInfoButtons],
		URLButton:     Defaults[FieldURLButton],
		AudioButton:   Defaults[FieldAudioButton],
		FileButton:    Defaults[FieldFileButton],
		VideoQuality:  Defaults[FieldVideoQuality],
		AsDocument:    Defaults[FieldAsDocument],
		AudioFormat:   Defaults[FieldAudioFormat],
	}
}

// Get returns the value of a single field.
func (s Set) Get(field string) string {
	switch field {
	case FieldCaptions:
		return s.Captions
	case FieldDeleteMessage:
		return s.DeleteMessage
	case FieldInfoButtons:
		return s.InfoButtons
	case FieldURLButton:
		return s.URLButton
	case FieldAudioButton:
		return s.AudioButton
	case FieldFileButton:
		return s.FileButton
	case FieldVideoQuality:
		return s.VideoQuality
	case FieldAsDocument:
		return s.AsDocument
	case FieldAudioFormat:
		return s.AudioFormat
	default:
		return ""
	}
}

// With returns a copy of the set with a single field replaced.
func (s Set) With(field, value string) Set {
	switch field {
	case FieldCaptions:
		s.Captions = value
	case FieldDeleteMessage:
		s.DeleteMessage = value
	case FieldInfoButtons:
		s.InfoButtons = value
	case FieldURLButton:
		s.URLButton = value
	case FieldAudioButton:
		s.AudioButton = value
	case FieldFileButton:
		s.FileButton = value
	case FieldVideoQuality:
		s.VideoQuality = value
	case FieldAsDocument:
		s.AsDocument = value
	case FieldAudioFormat:
		s.AudioFormat = value
	}
	return s
}

// IsOn reports whether a boolean setting is enabled.
func (s Set) IsOn(field string) bool { return s.Get(field) == On }

// Callback prefixes of the settings menu.
const (
	PrefixView     = "settings:"
	PrefixToggle   = "setting:"
	PrefixCategory = "settings_cat:"
)

// ParseViewCallback parses a "settings:<field>" callback.
func ParseViewCallback(data string) (string, bool) {
	if !strings.HasPrefix(data, "settings:") {
		return "", false
	}
	field := strings.TrimPrefix(data, "settings:")
	if !IsKnownField(field) {
		return "", false
	}
	return field, true
}

// ParseToggleCallback parses a "setting:<field>:<value>" callback.
func ParseToggleCallback(data string) (string, string, bool) {
	parts := strings.Split(data, ":")
	if len(parts) != 3 || parts[0] != "setting" {
		return "", "", false
	}
	field, value := parts[1], parts[2]
	if !IsKnownField(field) || !IsKnownValue(field, value) {
		return "", "", false
	}
	return field, value, true
}

// ParseCategoryCallback parses a "settings_cat:<category>" callback.
func ParseCategoryCallback(data string) (string, bool) {
	if !strings.HasPrefix(data, "settings_cat:") {
		return "", false
	}
	category := strings.TrimPrefix(data, "settings_cat:")
	for _, candidate := range Categories {
		if candidate == category {
			return category, true
		}
	}
	return "", false
}

// VideoFormatSelector maps the video quality preference onto a yt-dlp format
// selector string.
//
// The chain asks for the height of the preference first. A video that keeps the
// wanted height only in a newer codec is not downgraded to the lower stream
// of the older codec. Inside a height the h264 streams come first, they play
// everywhere. The progressive mp4 files are the last fallback, they are
// smaller but rarely go beyond 720p.
func VideoFormatSelector(quality string) string {
	switch quality {
	case Saver:
		return videoFormatSelector(480)
	case Balanced:
		return videoFormatSelector(720)
	case Ultra:
		return videoFormatSelector(2160)
	default:
		return videoFormatSelector(1080)
	}
}

// videoFormatSelector builds the format chain of a maximum height.
//
// The first three entries of the chain want the height of the preference
// exactly, the h264 stream comes first and the newer codecs follow. The
// entries below it take the best stream that is not higher than the
// preference, so a video without the wanted height still arrives.
func videoFormatSelector(height int) string {
	return fmt.Sprintf(
		"bestvideo[height=%d][vcodec^=avc1]+bestaudio[ext=m4a]/"+
			"bestvideo[height=%d][ext=mp4]+bestaudio[ext=m4a]/"+
			"bestvideo[height=%d]+bestaudio/"+
			"bestvideo[height<=%d][vcodec^=avc1]+bestaudio[ext=m4a]/"+
			"bestvideo[height<=%d]+bestaudio/"+
			"best[height<=%d][ext=mp4][acodec!=none][vcodec!=none]/"+
			"best[height<=%d][acodec!=none][vcodec!=none]",
		height, height, height, height, height, height, height,
	)
}

// VideoMaxHeight maps the video quality preference onto a maximum height.
func VideoMaxHeight(quality string) int {
	switch quality {
	case Saver:
		return 480
	case Balanced:
		return 720
	case Ultra:
		return 2160
	default:
		return 1080
	}
}

// AudioCodec maps the audio preference onto a container name.
func AudioCodec(value string) string {
	switch value {
	case M4A:
		return "m4a"
	case FLAC:
		return "flac"
	default:
		return "mp3"
	}
}

// AudioBitrate maps the audio preference onto a bitrate in kbit per second.
//
// A value of zero keeps the quality of the source, which is used by the
// lossless containers.
func AudioBitrate(value string) int {
	switch value {
	case MP3:
		return 320
	default:
		return 0
	}
}

// AudioFormatLabel renders the audio preference for the settings menu.
func AudioFormatLabel(value string) string {
	switch value {
	case MP3:
		return "MP3 320 kbit/s"
	case M4A:
		return "M4A (AAC, source quality)"
	case FLAC:
		return "FLAC (lossless)"
	default:
		return "MP3 320 kbit/s"
	}
}

// ToggleLabels returns the labels of the two states of a boolean setting.
//
// The stored value stays on or off, only the label of the button changes, so
// the options that switch something on read better than a plain on.
func ToggleLabels(field string) (string, string) {
	switch field {
	case FieldCaptions, FieldInfoButtons, FieldAudioButton, FieldFileButton, FieldURLButton:
		return "shown", "hidden"
	default:
		return On, Off
	}
}

// ValueLabel renders the current value of a setting.
//
// The settings list shows the value next to the name of the option, so the
// user sees the active choice without opening every option.
func ValueLabel(field string, value string) string {
	switch field {
	case FieldVideoQuality:
		return VideoQualityLabel(value)
	case FieldAudioFormat:
		return AudioFormatLabel(value)
	default:
		if value == On {
			on, _ := ToggleLabels(field)
			return on
		}
		_, off := ToggleLabels(field)
		return off
	}
}

// VideoQualities lists the video quality preferences from the highest
// resolution to the lowest one.
//
// The screens and the keyboards walk the list, so the order of the buttons
// and the order of the description are always the same.
func VideoQualities() []string {
	return []string{Ultra, Best, Balanced, Saver}
}

// VideoQualityLabel renders the video preference for the settings menu.
//
// The resolution comes first, it is the part the user looks for, the name of
// the preference follows it.
func VideoQualityLabel(value string) string {
	switch value {
	case Saver:
		return "480p (data saver)"
	case Balanced:
		return "720p (balanced)"
	case Ultra:
		return "2160p (4k ultra)"
	default:
		return "1080p (best)"
	}
}

// VideoQualityNote renders the short hint of a video preference.
//
// The hint is shown under the button of the preference inside the settings
// screen, it explains what the resolution costs.
func VideoQualityNote(value string) string {
	switch value {
	case Saver:
		return "the smallest file, the fastest download"
	case Balanced:
		return "less data, still sharp enough for a phone"
	case Ultra:
		return "the biggest file, it needs a fast line"
	default:
		return "the usual choice, it plays everywhere"
	}
}

// String renders the preference set for logging.
func (s Set) String() string {
	return fmt.Sprintf("captions=%s delete=%s info=%s url=%s audio=%s file=%s quality=%s document=%s format=%s",
		s.Captions, s.DeleteMessage, s.InfoButtons, s.URLButton, s.AudioButton, s.FileButton,
		s.VideoQuality, s.AsDocument, s.AudioFormat)
}
