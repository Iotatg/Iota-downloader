// Package messages holds every user facing string of the Iota Coder bot. All
// texts are rendered with the fancy unicode style of the bot.
package messages

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/Iotatg/iota-downloader/internal/settings"
	"github.com/Iotatg/iota-downloader/internal/text"
)

// BrandName is the display name of the bot.
const BrandName = "Iota Coder"

// BotName is the short name used inside the fancy brand line.
const BotName = "Iota"

// Banner returns the fancy brand line of the bot.
func Banner() string { return text.Banner() }

// FormatNumber renders a number in a compact form.
func FormatNumber(value int64) string {
	switch {
	case value >= 1_000_000_000:
		return fmt.Sprintf("%.1fB", float64(value)/1_000_000_000)
	case value >= 1_000_000:
		return fmt.Sprintf("%.1fM", float64(value)/1_000_000)
	case value >= 1_000:
		return fmt.Sprintf("%.1fK", float64(value)/1_000)
	default:
		return fmt.Sprintf("%d", value)
	}
}

// Layout helpers of the Iota Coder look.

// header renders the title of a screen.
func header(symbol, title string) string {
	if symbol == "" {
		symbol = text.SymBar
	}
	return "<b>" + symbol + " " + text.Bold(title) + "</b>"
}

// item renders a list entry of a screen.
func item(symbol, value string) string {
	if symbol == "" {
		symbol = text.SymBullet
	}
	return symbol + " " + text.Small(value)
}

// step renders a numbered step of a flow.
func step(number int, value string) string {
	return text.SymArrow + " " + fmt.Sprintf("%d", number) + " " + text.Small(value)
}

// group renders a group of platform names.
func group(names ...string) string {
	return text.SymNote + " " + text.Small(strings.Join(names, " • "))
}

// command renders a list entry that holds a shell command.
//
// The command itself stays in plain text, so it can be copied out of the
// message.
func command(symbol, label, value string) string {
	return symbol + " " + text.Small(label) + ": " + value
}

// rule renders the separator line of a screen.
func rule() string { return text.Rule(26) }

// Welcome is the greeting of the bot.
func Welcome(botUsername string) string {
	lines := []string{
		Banner(),
		"",
		header(text.SymBar, "your downloader"),
		item(text.SymLeft, "send me a link and i download the video, the photo or the track of it."),
		item(text.SymRight, "or paste several links into one message, i handle them one by one."),
		"",
		header(text.SymArrow, "how it works"),
		step(1, "you send the link of a post, a video or a track."),
		step(2, "i read the link and fetch the media of it."),
		step(3, "the file arrives here, in the chat, ready to be saved."),
		"",
		header(text.SymNote, "supported sites"),
		group("Instagram", "Threads", "TikTok"),
		group("YouTube", "YouTube Music", "X / Twitter"),
		group("SoundCloud", "Spotify", "Pinterest"),
		group("Apple Music", "JioSaavn", "Deezer", "Gaana", "Tidal"),
		group("Bluesky", "Facebook", "Reddit", "Twitch", "Kick"),
		group("Bilibili", "Tumblr", "LinkedIn", "IMDb"),
		group("MX Player", "Snapchat", "Mastodon", "Resso"),
		group("Shorts", "Reels", "IGTV", "Live posts"),
		"",
		rule(),
		"",
		item(text.SymMenu, "the menu below opens the settings, the statistics and the help."),
	}
	return strings.Join(lines, "\n")
}

// Help explains the supported sites and the usage of the bot.
func Help(botUsername string) string {
	lines := []string{
		Banner(),
		"",
		header(text.SymBar, "help & guide"),
		item(text.SymBullet, "send one link or paste multiple links in one message, the bot extracts and delivers the media."),
		"",
		header(text.SymLeft, "Instagram & Threads"),
		item(text.SymBullet, "download posts, reels, igtv & stories"),
		item(text.SymBullet, "photo carousels & multi media albums"),
		item(text.SymBullet, "copy the link with share to copy link"),
		"",
		header(text.SymRight, "TikTok"),
		item(text.SymBullet, "watermark free video downloads"),
		item(text.SymBullet, "photo carousels & slideshows"),
		item(text.SymBullet, "mp3 audio extraction supported"),
		"",
		header(text.SymUp, "YouTube & YouTube Music"),
		item(text.SymBullet, "shorts & regular videos"),
		item(text.SymBullet, "high quality audio & video streams"),
		item(text.SymBullet, "tap the mp3 button to download the audio"),
		"",
		header(text.SymForward, "X / Twitter & Pinterest"),
		item(text.SymBullet, "x videos, gifs & images"),
		item(text.SymBullet, "pinterest video and image pins"),
		"",
		header(text.SymUpLeft, "SoundCloud & Spotify"),
		item(text.SymBullet, "high quality soundcloud audio tracks"),
		item(text.SymBullet, "spotify track matching & audio download"),
		"",
		header(text.SymErase, "Apple Music, JioSaavn, Deezer, Gaana & Tidal"),
		item(text.SymBullet, "full length tracks of the music platforms"),
		item(text.SymBullet, "delivered with the cover of the track"),
		"",
		header(text.SymHex, "More platforms"),
		item(text.SymBullet, "bluesky, facebook, reddit, twitch, kick & bilibili"),
		item(text.SymBullet, "tumblr, linkedin, imdb, mx player, snapchat, mastodon & resso"),
		item(text.SymBullet, "videos and photos of the social networks"),
		"",
		header(text.SymArrow, "Inline Mode"),
		item(text.SymBullet, "type @"+botUsername+" [link] in any chat"),
		item(text.SymBullet, "instant preview and direct media sharing"),
		"",
		header(text.SymStarOut, "Batch Downloading"),
		item(text.SymBullet, "paste up to 6 links in a single message"),
		item(text.SymBullet, "delivered one by one to keep the chat clean"),
		"",
		rule(),
		"",
		item(text.SymInfo, "the media that is bigger than 50 MB is sent over mtproto, so the full size file arrives as one message."),
	}
	return strings.Join(lines, "\n")
}

// Settings is the entry text of the settings menu.
func Settings() string {
	return fmt.Sprintf(`%s
%s`, header(text.SymBar, "settings"), item(text.SymBullet, "use the buttons below to tune how the downloads are sent, the changes apply only to your account."))
}

// SettingsPrivateOnly is shown when the settings are opened in a group.
func SettingsPrivateOnly() string {
	return item(text.SymBullet, "settings are available only in a private chat, open a direct message with the bot to change the preferences.")
}

// SettingsAdminOnly is shown when a non admin opens the settings in a group.
func SettingsAdminOnly() string {
	return item(text.SymBullet, "only group admins can open the settings in group chats.")
}

// CategorySettingsText returns the header of a settings category.
func CategorySettingsText(category string) string {
	switch category {
	case settings.CategoryMedia:
		return fmt.Sprintf(`%s

%s`, header(text.SymLeft, "media & quality settings"), item(text.SymBullet, "configure the video resolution, the file format and the audio options."))
	case settings.CategoryAppearance:
		return fmt.Sprintf(`%s

%s`, header(text.SymRight, "appearance & buttons"), item(text.SymBullet, "customize the post descriptions, the original url link and the action buttons."))
	case settings.CategoryChat:
		return fmt.Sprintf(`%s

%s`, header(text.SymUp, "chat & clean up"), item(text.SymBullet, "manage the group chat behaviour and the message clean up."))
	default:
		return Settings()
	}
}

// FieldText returns the description of a single setting.
func FieldText(field string) string {
	switch field {
	case settings.FieldCaptions:
		return fmt.Sprintf(`%s
%s`, header(text.SymRight, "descriptions"), item(text.SymBullet, "show or hide the post captions in the downloaded media, some sources do not provide captions."))
	case settings.FieldDeleteMessage:
		return fmt.Sprintf(`%s
%s`, header(text.SymDelete, "delete messages"), item(text.SymBullet, "remove your link automatically once the download is handled."))
	case settings.FieldInfoButtons:
		return fmt.Sprintf(`%s
%s`, header(text.SymStar, "info buttons"), item(text.SymBullet, "toggle the additional info buttons under the downloaded media."))
	case settings.FieldURLButton:
		return fmt.Sprintf(`%s
%s`, header(text.SymForward, "url button"), item(text.SymBullet, "show or hide the button with the original post link."))
	case settings.FieldAudioButton:
		return fmt.Sprintf(`%s
%s`, header(text.SymUp, "mp3 button"), item(text.SymBullet, "toggle the download mp3 button when audio is available."))
	case settings.FieldFileButton:
		return fmt.Sprintf(`%s
%s`, header(text.SymLeft, "file button"), item(text.SymBullet, "show or hide the download file button under the videos to get the original uncompressed files on demand."))
	case settings.FieldVideoQuality:
		return fmt.Sprintf(`%s
%s

%s
%s
%s
%s`,
			header(text.SymUp, "video quality"),
			item(text.SymBullet, "pick the resolution of the video downloads, the streams that are not available are skipped."),
			item(text.SymBullet, settings.VideoQualityLabel(settings.Ultra)+" - "+settings.VideoQualityNote(settings.Ultra)+"."),
			item(text.SymBullet, settings.VideoQualityLabel(settings.Best)+" - "+settings.VideoQualityNote(settings.Best)+"."),
			item(text.SymBullet, settings.VideoQualityLabel(settings.Balanced)+" - "+settings.VideoQualityNote(settings.Balanced)+"."),
			item(text.SymBullet, settings.VideoQualityLabel(settings.Saver)+" - "+settings.VideoQualityNote(settings.Saver)+"."),
		)
	case settings.FieldAsDocument:
		return fmt.Sprintf(`%s
%s`, header(text.SymLeft, "send as file"), item(text.SymBullet, "when enabled the videos and photos are sent as uncompressed documents (.mp4 / .jpg) with 100% original quality."))
	case settings.FieldAudioFormat:
		return fmt.Sprintf(`%s
%s

%s
%s
%s`,
			header(text.SymUp, "audio format"),
			item(text.SymBullet, "choose the default audio format for the music downloads."),
			item(text.SymBullet, settings.AudioFormatLabel(settings.MP3)+" - the safe choice, it plays everywhere."),
			item(text.SymBullet, settings.AudioFormatLabel(settings.M4A)+" - the quality of the source in a small file."),
			item(text.SymBullet, settings.AudioFormatLabel(settings.FLAC)+" - uncompressed, the biggest file."),
		)
	default:
		return fmt.Sprintf(`%s
%s`, header(text.SymBar, "settings"), item(text.SymBullet, "this option does not have a description yet."))
	}
}

// Caption builds the caption of a delivered media item.
//
// The caption carries the platform, the author and the description of the post
// before the brand line. The description is only shown when the user enabled
// it in the settings.
func Caption(userCaptions, postCaption, botURL string, limit int) string {
	return MediaCaption(userCaptions, "", "", postCaption, botURL, limit)
}

// MediaCaption builds the caption of a media item with its platform header.
func MediaCaption(userCaptions, service, author, postCaption, botURL string, limit int) string {
	footer := fmt.Sprintf("%s %s <a href=\"%s\">%s</a>", text.SymForward, text.Small("powered by"), botURL, text.Bold(BotName))
	if limit <= 0 {
		limit = 1024
	}

	header := ""
	if service != "" {
		header = text.Title(service)
		if author != "" {
			header += " " + text.Small("•") + " " + text.Small("@"+author)
		}
		header += "\n"
	}

	if userCaptions != settings.On || strings.TrimSpace(postCaption) == "" {
		if header == "" {
			return footer
		}
		return header + footer
	}

	body := []rune(EscapeHTML(postCaption))
	footerRunes := []rune(footer)
	headerRunes := []rune(header)
	if len(body)+len(footerRunes)+len(headerRunes)+2 > limit {
		available := limit - len(footerRunes) - len(headerRunes) - 3
		if available < 0 {
			available = 0
		}
		if len(body) > available {
			body = append(body[:available], '…')
		}
	}
	if header == "" {
		return string(body) + "\n\n" + footer
	}
	return header + string(body) + "\n\n" + footer
}

// EscapeHTML escapes the characters that would break the HTML parse mode.
func EscapeHTML(value string) string {
	replacer := strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;")
	return replacer.Replace(value)
}

// Status messages.

func DownloadingAudioStatus() string { return item(text.SymWait, "downloading the audio...") }
func DownloadingVideoStatus() string { return item(text.SymWait, "downloading the video...") }
func UploadingStatus() string        { return item(text.SymWait, "uploading the file to telegram...") }

// ResolvingStatus is sent the moment a link arrives, so the user never waits
// in silence while the platform is being asked.
func ResolvingStatus(service string) string {
	if service == "" {
		return item(text.SymWait, "reading the link...")
	}
	return item(text.SymWait, "reading the "+strings.ToLower(service)+" link...")
}

// PreparingStatus is sent while the media is fetched.
func PreparingStatus() string { return item(text.SymWait, "preparing the media...") }

// DoneStatus closes a finished job.
func DoneStatus() string { return item(text.SymCheck, "done") }

// SplitPartCaption explains that a file was too big for a single upload and
// was split into parts.
func SplitPartCaption(caption string, parts int, original string) string {
	name := filepath.Base(original)
	lines := make([]string, 0, 5)
	if caption != "" {
		lines = append(lines, caption, "")
	}
	lines = append(lines,
		header(text.SymSplit, "the file was too big for telegram"),
		item(text.SymBullet, fmt.Sprintf("it is split into %d parts, download all of them to rebuild the file.", parts)),
		command(text.SymBullet, "linux", EscapeHTML(fmt.Sprintf("cat %s.part* > %s", name, name))),
		command(text.SymBullet, "windows", EscapeHTML(fmt.Sprintf("copy /b %s.part* %s", name, name))),
	)
	return strings.Join(lines, "\n")
}

func DMStartRequired() string {
	return text.SymErase + " " + text.Small("first time setup needed: open a direct message, press start and send the link again.")
}
func DuplicateLinkProcessing() string {
	return item(text.SymStar, "this link is already being processed, wait a few seconds.")
}
func DuplicateLinkRecentlyProcessed() string {
	return item(text.SymStar, "this link was just handled, send it again in a few seconds if you still need it.")
}
func InvalidSettingsOption() string { return item(text.SymCross, "invalid settings option.") }
func JoinGroup(chatTitle string) string {
	return fmt.Sprintf(`%s
%s
%s
%s`,
		item(text.SymCheck, "thanks for adding "+BotName+" to"),
		item(text.SymCheck, "the group "+EscapeHTML(chatTitle)),
		item(text.SymBullet, "please grant the admin rights to unlock the full functionality"),
		item(text.SymBullet, "the bot needs the rights to delete and to pin its own messages"),
	)
}
func AdminRightsGranted(chatTitle string) string {
	return fmt.Sprintf(`%s
%s`,
		item(text.SymCheck, "admin rights granted in "+EscapeHTML(chatTitle)),
		item(text.SymBullet, "the downloads keep running smoothly now."),
	)
}
func KeyboardRemoved() string { return item(text.SymCheck, "reply keyboard removed.") }
func TikTokLiveNotSupported() string {
	return item(text.SymCross, "tiktok live streams are not supported yet, send a regular tiktok post link.")
}
func DeletePermissionWarning() string {
	return item(text.SymCross, "auto delete failed: the bot needs the permission to delete messages in this chat, grant it or turn the auto delete off in the settings.")
}
func StatsTemporarilyUnavailable() string {
	return item(text.SymCross, "the statistics could not be generated right now, please try again later.")
}
func NoQueueMetricsYet() string { return item(text.SymBullet, "no queue metrics yet.") }
func OpenBotForAudio() string {
	return item(text.SymBullet, "open the bot in a private chat to download the audio.")
}
func AudioFetchFailed() string {
	return item(text.SymCross, "the audio info could not be read, please try again later.")
}
func AudioDownloadFailed() string {
	return item(text.SymCross, "the audio download failed, please try again later.")
}
func SpotifyMetadataFailed() string {
	return item(text.SymCross, "this spotify track could not be read, check the link and try again.")
}
func SpotifySourceNotFound() string {
	return item(text.SymCross, "no matching audio source was found for this spotify track.")
}
func InlineAlbumLinkInvalid() string {
	return item(text.SymCross, "this album link is expired or invalid.")
}
func InlinePhotoTitle(service string) string {
	return fmt.Sprintf("%s %s", text.Small(service), text.Small("photo"))
}
func InlinePhotoDescription() string { return text.Small("single photo") }
func InlineAlbumTitle(service string) string {
	return fmt.Sprintf("%s %s", text.Small(service), text.Small("album"))
}
func InlineAlbumDescription() string    { return text.Small("open the full album in the bot") }
func InlineOpenFullAlbumButton() string { return text.Small("open the full album") }
func InlineSendVideoButton() string     { return text.Small("send the video inline") }
func InlineSendVideoPrompt(service string) string {
	return fmt.Sprintf("%s\n%s", item(text.SymWait, strings.ToLower(service)+" video is being prepared"), item(text.SymBullet, "if it does not start automatically, tap the button below."))
}
func InlineSendAudioPrompt(service string) string {
	return fmt.Sprintf("%s\n%s", item(text.SymWait, strings.ToLower(service)+" audio is being prepared"), item(text.SymBullet, "if it does not start automatically, tap the button below."))
}
func InlineVideoAlreadyProcessing() string {
	return item(text.SymStar, "this inline video is already being prepared.")
}
func InlineVideoAlreadySent() string {
	return item(text.SymCheck, "this inline video was already sent.")
}
func InlinePhotosNotSupported(service string) string {
	return item(text.SymCross, strings.ToLower(service)+" photos are not supported inline.")
}

// Batch messages.

func BatchLinksStarted(processedTotal, detectedTotal int) string {
	if detectedTotal > processedTotal {
		return fmt.Sprintf("%s\n%s", item(text.SymBullet, fmt.Sprintf("found %d supported links, the first %d are processed one by one so the chat stays readable.", detectedTotal, processedTotal)), item(text.SymBullet, "the rest of the links are ignored, send them again if you need them."))
	}
	return item(text.SymBullet, fmt.Sprintf("found %d supported links, they are processed one by one so the chat stays readable.", processedTotal))
}

func BatchLinkProgress(current, total int, serviceName string) string {
	return item(text.SymWait, fmt.Sprintf("link %d/%d: %s", current, total, strings.ToLower(serviceName)))
}

func BatchLinksFinished(total int) string {
	return item(text.SymCheck, fmt.Sprintf("batch finished, %d links processed.", total))
}

// Error messages.

func TimeoutError() string {
	return item(text.SymCross, "the request timed out, the source may be slow right now, please try again later.")
}
func SomethingWentWrong() string {
	return item(text.SymCross, "this link could not be processed right now.") + "\n" +
		item(text.SymBullet, "it may be private, deleted, region limited or temporarily blocked by the source, please try again later.")
}

// InstagramSessionMissing explains that Instagram needs a logged in session.
func InstagramSessionMissing() string {
	return item(text.SymCross, "instagram refused the request.") + "\n" +
		item(text.SymBullet, "instagram only answers logged in sessions: set instagram_sessionid or instagram_cookies_file on the server and try again.")
}

// ThreadsSessionMissing explains that Threads needs a logged in session.
func ThreadsSessionMissing() string {
	return item(text.SymCross, "threads refused the request.") + "\n" +
		item(text.SymBullet, "threads only answers logged in sessions: set instagram_sessionid or instagram_cookies_file on the server and try again.")
}

// LinkFailed builds the answer of a link that could not be resolved.
//
// Some platforms need a login session or a configured backend, so the answer
// names the setting that is missing instead of a generic apology.
func LinkFailed(service string) string {
	switch service {
	case "instagram":
		return InstagramSessionMissing()
	case "threads":
		return ThreadsSessionMissing()
	case "pinterest":
		return item(text.SymCross, "this platform is not available yet.") + "\n" +
			item(text.SymBullet, "it needs a cobalt backend: set cobalt_api_url and cobalt_api_key on the server and try again.")
	case "soundcloud":
		return item(text.SymCross, "the soundcloud track was not found.") + "\n" +
			item(text.SymBullet, "the music backend of the server must answer: set iota_api_url and iota_api_key, start the api and try again.")
	case "reddit", "facebook", "tumblr":
		return item(text.SymCross, "this platform refused the request.") + "\n" +
			item(text.SymBullet, "it asks for a logged in session: set ytdlp_cookies_file on the server and try again.")
	case "spotify":
		return item(text.SymCross, "the music backend did not answer.") + "\n" +
			item(text.SymBullet, "the iota api of the server is offline: set iota_api_url and iota_api_key, start the api and try again.")
	default:
		return SomethingWentWrong()
	}
}
func VideoTooLarge() string {
	return item(text.SymCross, "the video is bigger than the size limit of the bot, try a shorter video or the mp3 option if it is available.")
}
func AudioTooLarge() string {
	return item(text.SymCross, "the audio is bigger than the size limit of the bot, try a shorter track or another source link.")
}
func NothingFound() string {
	return item(text.SymCross, "no media found, check that the link is public, not expired and points directly to a post or a video.")
}

// Guest mode.

func GuestHelpMessage(botUsername string) string {
	lines := []string{
		Banner(),
		"",
		header(text.SymBar, "guest mode"),
		item(text.SymBullet, "you summoned the bot in this chat, mention it with a link of a supported platform to download the media."),
		"",
		header(text.SymNote, "the platforms that work here"),
		group("TikTok", "Instagram", "Threads", "YouTube"),
		group("X / Twitter", "SoundCloud", "Spotify", "Pinterest"),
		group("Bluesky", "Facebook", "Reddit", "Twitch", "Kick", "Bilibili & more"),
		"",
		rule(),
		"",
		item(text.SymMenu, "add the bot to this group or open a direct chat with @"+botUsername+" for the full features and the settings."),
	}
	return strings.Join(lines, "\n")
}

func GuestDownloadingStatus(serviceName string) string {
	return fmt.Sprintf("%s\n%s", item(text.SymWait, strings.ToLower(serviceName)+" media is being prepared"), item(text.SymBullet, "please wait a moment while the media is processed."))
}

func GuestUnsupportedLink() string {
	return item(text.SymCross, "no supported media link was found in your mention, include a link of one of the supported platforms.")
}

// Progress and queue texts.

func RateLimitText(retryAfter int) string {
	return fmt.Sprintf("%s\n%s",
		item(text.SymStar, "too many requests from your account right now."),
		item(text.SymBullet, fmt.Sprintf("please wait %d seconds and try again.", retryAfter)),
	)
}

func QueueBusyText(position int) string {
	return fmt.Sprintf("%s\n%s",
		item(text.SymStar, "the download queue is busy right now."),
		item(text.SymBullet, fmt.Sprintf("your request would be around position #%d.", position)),
	)
}

func QueueStatusText(label string, position, activeWorkers int) string {
	return fmt.Sprintf("%s\n%s\n%s",
		item(text.SymStar, "queueing "+strings.ToLower(label)+"..."),
		item(text.SymBullet, fmt.Sprintf("position: %d", position)),
		item(text.SymBullet, fmt.Sprintf("workers: %d", activeWorkers)),
	)
}

// ProgressText renders the download progress bar.
func ProgressText(label string, downloaded, total int64, speed float64, etaSeconds float64) string {
	header := text.Label(text.SymWait, label)
	if total <= 0 {
		return fmt.Sprintf("%s\n%s • %s • %s", header,
			FormatBytes(downloaded), FormatSpeed(speed), FormatETA(etaSeconds))
	}
	percent := float64(downloaded) / float64(total) * 100
	bar := ProgressBar(percent, 10)
	return fmt.Sprintf("%s\n%s %.0f%%\n%s / %s • %s • %s", header, bar, percent,
		FormatBytes(downloaded), FormatBytes(total), FormatSpeed(speed), FormatETA(etaSeconds))
}

// ProgressBar renders a textual progress bar.
func ProgressBar(percent float64, width int) string {
	return text.Bar(percent, width)
}

// FormatBytes renders a byte count in a human readable form.
func FormatBytes(size int64) string {
	if size < 0 {
		size = 0
	}
	const unit = 1024
	if size < unit {
		return fmt.Sprintf("%d B", size)
	}
	div, exp := int64(unit), 0
	for n := size / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.2f %cB", float64(size)/float64(div), "KMGTPE"[exp])
}

// FormatSpeed renders a transfer speed.
func FormatSpeed(bytesPerSecond float64) string {
	if bytesPerSecond <= 0 {
		return "0 B/s"
	}
	const unit = 1024
	if bytesPerSecond < unit {
		return fmt.Sprintf("%.0f B/s", bytesPerSecond)
	}
	div, exp := float64(unit), 0
	for size := bytesPerSecond / unit; size >= unit; size /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB/s", bytesPerSecond/div, "KMGTPE"[exp])
}

// FormatETA renders the remaining time of a download.
func FormatETA(seconds float64) string {
	if seconds <= 0 {
		return "—"
	}
	if seconds < 60 {
		return fmt.Sprintf("%.0fs", seconds)
	}
	if seconds < 3600 {
		return fmt.Sprintf("%dm %ds", int(seconds)/60, int(seconds)%60)
	}
	return fmt.Sprintf("%dh %dm", int(seconds)/3600, (int(seconds)%3600)/60)
}
