package platforms

import (
	"net/url"
	"regexp"
	"strings"
)

// Extra platforms served by yt-dlp (and, for music, by the Ritesh api
// when it answers). Named hosts give a clear label; everything else
// that yt-dlp knows is caught by ServiceGeneric.
const (
	ServiceVimeo         = "vimeo"
	ServiceBandcamp      = "bandcamp"
	ServiceMixcloud      = "mixcloud"
	ServiceVK            = "vk"
	ServiceRumble        = "rumble"
	ServiceStreamable    = "streamable"
	ServiceAudiomack     = "audiomack"
	ServiceOdnoklassniki = "odnoklassniki"
	ServiceCoub          = "coub"
	ServiceDouyin        = "douyin"
	ServiceKuaishou      = "kuaishou"
	ServiceTriller       = "triller"
	ServiceNiconico      = "niconico"
	ServiceOdysee        = "odysee"
	ServiceNineGag       = "ninegag"
	ServiceLikee         = "likee"
	ServiceNewgrounds    = "newgrounds"
	ServiceFlickr        = "flickr"
	ServiceIzlesene      = "izlesene"
	ServiceSmugMug       = "smugmug"
	ServiceBitChute      = "bitchute"
	ServiceVeoh          = "veoh"
	ServiceImgur         = "imgur"
	ServiceLoom          = "loom"
	ServiceWistia        = "wistia"
	ServiceTrovo         = "trovo"
	ServiceWeibo         = "weibo"
	ServiceRutube        = "rutube"
	ServiceTED           = "ted"
	ServiceArchive       = "archive"
	ServiceHearThis      = "hearthis"
	ServiceJamendo       = "jamendo"
	ServiceReverbNation  = "reverbnation"
	ServiceAudius        = "audius"
	ServiceBoomplay      = "boomplay"
	ServiceAnghami       = "anghami"
	ServiceWynk          = "wynk"
	ServiceHungama       = "hungama"
	ServiceYouku         = "youku"
	ServiceIQIYI         = "iqiyi"
	ServiceXiaohongshu   = "xiaohongshu"
	ServicePeerTube      = "peertube"
	ServiceAfreecaTV     = "afreecatv"
	ServiceArtStation    = "artstation"
	ServiceDeviantArt    = "deviantart"
	ServicePixiv         = "pixiv"
	ServiceBehance       = "behance"
	ServiceDribbble      = "dribbble"
	ServiceUnsplash      = "unsplash"
	ServicePexels        = "pexels"
	ServicePixabay       = "pixabay"
	ServiceGiphy         = "giphy"
	ServiceTenor         = "tenor"
	ServiceNapster       = "napster"
	ServiceKugou         = "kugou"
	ServiceKuwo          = "kuwo"
	ServiceQQMusic       = "qqmusic"
	ServiceNetEase       = "netease"
	ServiceRedBullTV     = "redbulltv"
	ServiceIHeartRadio   = "iheartradio"
	ServiceTuneIn        = "tunein"
	ServicePandora       = "pandora"
	ServiceLastFM        = "lastfm"
	ServiceGenius        = "genius"
	ServiceTerabox       = "terabox"

	// ServiceGeneric is any other page that yt-dlp can extract.
	ServiceGeneric = "web"
)

// ExtraAudioServices lists extra platforms that prefer an audio file.
var ExtraAudioServices = []string{
	ServiceBandcamp,
	ServiceMixcloud,
	ServiceAudiomack,
	ServiceHearThis,
	ServiceJamendo,
	ServiceReverbNation,
	ServiceAudius,
	ServiceBoomplay,
	ServiceAnghami,
	ServiceWynk,
	ServiceHungama,
	ServiceNapster,
	ServiceKugou,
	ServiceKuwo,
	ServiceQQMusic,
	ServiceNetEase,
	ServiceIHeartRadio,
	ServiceTuneIn,
	ServicePandora,
	ServiceLastFM,
	ServiceGenius,
}

// ExtraVideoServices lists extra platforms that prefer a video or a photo.
var ExtraVideoServices = []string{
	ServiceVimeo,
	ServiceVK,
	ServiceRumble,
	ServiceStreamable,
	ServiceOdnoklassniki,
	ServiceCoub,
	ServiceDouyin,
	ServiceKuaishou,
	ServiceTriller,
	ServiceNiconico,
	ServiceOdysee,
	ServiceNineGag,
	ServiceLikee,
	ServiceNewgrounds,
	ServiceFlickr,
	ServiceIzlesene,
	ServiceSmugMug,
	ServiceBitChute,
	ServiceVeoh,
	ServiceImgur,
	ServiceLoom,
	ServiceWistia,
	ServiceTrovo,
	ServiceWeibo,
	ServiceRutube,
	ServiceTED,
	ServiceArchive,
	ServiceYouku,
	ServiceIQIYI,
	ServiceXiaohongshu,
	ServicePeerTube,
	ServiceAfreecaTV,
	ServiceArtStation,
	ServiceDeviantArt,
	ServicePixiv,
	ServiceBehance,
	ServiceDribbble,
	ServiceUnsplash,
	ServicePexels,
	ServicePixabay,
	ServiceGiphy,
	ServiceTenor,
	ServiceRedBullTV,
	ServiceTerabox,
}

var extraAudioSet = map[string]bool{}
var extraVideoSet = map[string]bool{}
var extraLabel = map[string]string{
	ServiceVimeo:         "Vimeo",
	ServiceBandcamp:      "Bandcamp",
	ServiceMixcloud:      "Mixcloud",
	ServiceVK:            "VK",
	ServiceRumble:        "Rumble",
	ServiceStreamable:    "Streamable",
	ServiceAudiomack:     "Audiomack",
	ServiceOdnoklassniki: "OK.ru",
	ServiceCoub:          "Coub",
	ServiceDouyin:        "Douyin",
	ServiceKuaishou:      "Kuaishou",
	ServiceTriller:       "Triller",
	ServiceNiconico:      "Niconico",
	ServiceOdysee:        "Odysee",
	ServiceNineGag:       "9GAG",
	ServiceLikee:         "Likee",
	ServiceNewgrounds:    "Newgrounds",
	ServiceFlickr:        "Flickr",
	ServiceIzlesene:      "Izlesene",
	ServiceSmugMug:       "SmugMug",
	ServiceBitChute:      "BitChute",
	ServiceVeoh:          "Veoh",
	ServiceImgur:         "Imgur",
	ServiceLoom:          "Loom",
	ServiceWistia:        "Wistia",
	ServiceTrovo:         "Trovo",
	ServiceWeibo:         "Weibo",
	ServiceRutube:        "Rutube",
	ServiceTED:           "TED",
	ServiceArchive:       "Internet Archive",
	ServiceHearThis:      "HearThis.at",
	ServiceJamendo:       "Jamendo",
	ServiceReverbNation:  "ReverbNation",
	ServiceAudius:        "Audius",
	ServiceBoomplay:      "Boomplay",
	ServiceAnghami:       "Anghami",
	ServiceWynk:          "Wynk",
	ServiceHungama:       "Hungama",
	ServiceYouku:         "Youku",
	ServiceIQIYI:         "iQIYI",
	ServiceXiaohongshu:   "Xiaohongshu",
	ServicePeerTube:      "PeerTube",
	ServiceAfreecaTV:     "AfreecaTV",
	ServiceArtStation:    "ArtStation",
	ServiceDeviantArt:    "DeviantArt",
	ServicePixiv:         "Pixiv",
	ServiceBehance:       "Behance",
	ServiceDribbble:      "Dribbble",
	ServiceUnsplash:      "Unsplash",
	ServicePexels:        "Pexels",
	ServicePixabay:       "Pixabay",
	ServiceGiphy:         "Giphy",
	ServiceTenor:         "Tenor",
	ServiceNapster:       "Napster",
	ServiceKugou:         "Kugou",
	ServiceKuwo:          "Kuwo",
	ServiceQQMusic:       "QQ Music",
	ServiceNetEase:       "NetEase",
	ServiceRedBullTV:     "Red Bull TV",
	ServiceIHeartRadio:   "iHeartRadio",
	ServiceTuneIn:        "TuneIn",
	ServicePandora:       "Pandora",
	ServiceLastFM:        "Last.fm",
	ServiceGenius:        "Genius",
	ServiceTerabox:       "Terabox",
	ServiceGeneric:       "Web",
}

var extraPatterns = []servicePattern{
	{ServiceVimeo, regexp.MustCompile(`https?://(?:www\.|player\.)?vimeo\.com/\S+`)},
	{ServiceBandcamp, regexp.MustCompile(`https?://(?:[\w-]+\.)?bandcamp\.com/\S+`)},
	{ServiceMixcloud, regexp.MustCompile(`https?://(?:www\.)?mixcloud\.com/\S+`)},
	{ServiceVK, regexp.MustCompile(`https?://(?:[\w-]+\.)?vk\.com/\S+|https?://(?:[\w-]+\.)?vkvideo\.ru/\S+|https?://(?:[\w-]+\.)?vk\.ru/\S+`)},
	{ServiceRumble, regexp.MustCompile(`https?://(?:www\.)?rumble\.com/\S+`)},
	{ServiceStreamable, regexp.MustCompile(`https?://(?:www\.)?streamable\.com/\S+`)},
	{ServiceAudiomack, regexp.MustCompile(`https?://(?:www\.)?audiomack\.com/\S+`)},
	{ServiceOdnoklassniki, regexp.MustCompile(`https?://(?:www\.)?(?:ok|odnoklassniki)\.ru/\S+`)},
	{ServiceCoub, regexp.MustCompile(`https?://(?:www\.)?coub\.com/\S+`)},
	{ServiceDouyin, regexp.MustCompile(`https?://(?:www\.|v\.)?douyin\.com/\S+`)},
	{ServiceKuaishou, regexp.MustCompile(`https?://(?:www\.|v\.)?kuaishou\.com/\S+`)},
	{ServiceTriller, regexp.MustCompile(`https?://(?:www\.)?triller\.co/\S+`)},
	{ServiceNiconico, regexp.MustCompile(`https?://(?:www\.|sp\.)?nicovideo\.jp/\S+|https?://nico\.ms/\S+`)},
	{ServiceOdysee, regexp.MustCompile(`https?://(?:www\.)?odysee\.com/\S+`)},
	{ServiceNineGag, regexp.MustCompile(`https?://(?:www\.)?9gag\.com/\S+`)},
	{ServiceLikee, regexp.MustCompile(`https?://(?:www\.|l\.)?likee\.video/\S+|https?://(?:www\.)?likeevideo\.com/\S+`)},
	{ServiceNewgrounds, regexp.MustCompile(`https?://(?:www\.)?newgrounds\.com/\S+`)},
	{ServiceFlickr, regexp.MustCompile(`https?://(?:www\.)?flickr\.com/\S+|https?://flic\.kr/\S+`)},
	{ServiceIzlesene, regexp.MustCompile(`https?://(?:www\.)?izlesene\.com/\S+`)},
	{ServiceSmugMug, regexp.MustCompile(`https?://(?:[\w-]+\.)?smugmug\.com/\S+`)},
	{ServiceBitChute, regexp.MustCompile(`https?://(?:www\.)?bitchute\.com/\S+`)},
	{ServiceVeoh, regexp.MustCompile(`https?://(?:www\.)?veoh\.com/\S+`)},
	{ServiceImgur, regexp.MustCompile(`https?://(?:[\w-]+\.)?imgur\.com/\S+`)},
	{ServiceLoom, regexp.MustCompile(`https?://(?:www\.)?loom\.com/\S+`)},
	{ServiceWistia, regexp.MustCompile(`https?://(?:[\w-]+\.)?wistia\.com/\S+|https?://(?:[\w-]+\.)?wi\.st/\S+`)},
	{ServiceTrovo, regexp.MustCompile(`https?://(?:www\.)?trovo\.live/\S+`)},
	{ServiceWeibo, regexp.MustCompile(`https?://(?:[\w-]+\.)?weibo\.com/\S+|https?://(?:[\w-]+\.)?weibo\.cn/\S+`)},
	{ServiceRutube, regexp.MustCompile(`https?://(?:www\.)?rutube\.ru/\S+`)},
	{ServiceTED, regexp.MustCompile(`https?://(?:www\.)?ted\.com/(?:talks|watch)/\S+`)},
	{ServiceArchive, regexp.MustCompile(`https?://(?:www\.)?archive\.org/\S+`)},
	{ServiceHearThis, regexp.MustCompile(`https?://(?:www\.)?hearthis\.at/\S+`)},
	{ServiceJamendo, regexp.MustCompile(`https?://(?:www\.)?jamendo\.com/\S+`)},
	{ServiceReverbNation, regexp.MustCompile(`https?://(?:www\.)?reverbnation\.com/\S+`)},
	{ServiceAudius, regexp.MustCompile(`https?://(?:www\.)?audius\.co/\S+`)},
	{ServiceBoomplay, regexp.MustCompile(`https?://(?:www\.)?boomplay\.com/\S+`)},
	{ServiceAnghami, regexp.MustCompile(`https?://(?:www\.|play\.)?anghami\.com/\S+`)},
	{ServiceWynk, regexp.MustCompile(`https?://(?:www\.)?wynk\.in/\S+`)},
	{ServiceHungama, regexp.MustCompile(`https?://(?:www\.)?hungama\.com/\S+`)},
	{ServiceYouku, regexp.MustCompile(`https?://(?:[\w-]+\.)?youku\.com/\S+`)},
	{ServiceIQIYI, regexp.MustCompile(`https?://(?:www\.)?iqiyi\.com/\S+`)},
	{ServiceXiaohongshu, regexp.MustCompile(`https?://(?:www\.)?xiaohongshu\.com/\S+|https?://(?:www\.)?xhslink\.com/\S+`)},
	{ServicePeerTube, regexp.MustCompile(`https?://(?:[\w-]+\.)*(?:peertube|framatube|tilvids)[\w.-]*/\S+`)},
	{ServiceAfreecaTV, regexp.MustCompile(`https?://(?:www\.)?afreecatv\.com/\S+|https?://vod\.afreecatv\.com/\S+|https?://(?:www\.)?sooplive\.com/\S+`)},
	{ServiceArtStation, regexp.MustCompile(`https?://(?:www\.)?artstation\.com/\S+`)},
	{ServiceDeviantArt, regexp.MustCompile(`https?://(?:www\.)?deviantart\.com/\S+`)},
	{ServicePixiv, regexp.MustCompile(`https?://(?:www\.)?pixiv\.net/\S+`)},
	{ServiceBehance, regexp.MustCompile(`https?://(?:www\.)?behance\.net/\S+`)},
	{ServiceDribbble, regexp.MustCompile(`https?://(?:www\.)?dribbble\.com/\S+`)},
	{ServiceUnsplash, regexp.MustCompile(`https?://(?:www\.)?unsplash\.com/\S+`)},
	{ServicePexels, regexp.MustCompile(`https?://(?:www\.)?pexels\.com/\S+`)},
	{ServicePixabay, regexp.MustCompile(`https?://(?:www\.)?pixabay\.com/\S+`)},
	{ServiceGiphy, regexp.MustCompile(`https?://(?:[\w-]+\.)?giphy\.com/\S+`)},
	{ServiceTenor, regexp.MustCompile(`https?://(?:www\.)?tenor\.com/\S+`)},
	{ServiceNapster, regexp.MustCompile(`https?://(?:[\w-]+\.)?napster\.com/\S+`)},
	{ServiceKugou, regexp.MustCompile(`https?://(?:[\w-]+\.)?kugou\.com/\S+`)},
	{ServiceKuwo, regexp.MustCompile(`https?://(?:[\w-]+\.)?kuwo\.cn/\S+`)},
	{ServiceQQMusic, regexp.MustCompile(`https?://y\.qq\.com/\S+`)},
	{ServiceNetEase, regexp.MustCompile(`https?://(?:(?:music|y)\.)?163\.com/\S+`)},
	{ServiceRedBullTV, regexp.MustCompile(`https?://(?:www\.)?redbull\.com/\S+|https?://(?:www\.)?redbulltv\.com/\S+`)},
	{ServiceIHeartRadio, regexp.MustCompile(`https?://(?:www\.)?iheart\.com/\S+`)},
	{ServiceTuneIn, regexp.MustCompile(`https?://(?:www\.)?tunein\.com/\S+|https?://tun\.in/\S+`)},
	{ServicePandora, regexp.MustCompile(`https?://(?:www\.)?pandora\.com/\S+`)},
	{ServiceLastFM, regexp.MustCompile(`https?://(?:www\.)?last\.fm/\S+`)},
	{ServiceGenius, regexp.MustCompile(`https?://(?:[\w-]+\.)?genius\.com/\S+`)},
	{ServiceTerabox, regexp.MustCompile(`https?://(?:www\.)?(?:terabox|1024tera|teraboxapp)\.com/\S+`)},
}

var genericURLRe = regexp.MustCompile(`https?://[^\s<>"']+`)

var genericSkipHosts = map[string]bool{
	"google.com": true, "www.google.com": true, "maps.google.com": true,
	"wikipedia.org": true, "www.wikipedia.org": true, "en.wikipedia.org": true,
	"github.com": true, "gitlab.com": true, "bitbucket.org": true,
	"t.me": true, "telegram.me": true, "telegram.org": true,
	"wa.me": true, "whatsapp.com": true, "www.whatsapp.com": true,
	"discord.com": true, "discord.gg": true,
	"stackoverflow.com": true, "stackexchange.com": true,
	"microsoft.com": true, "apple.com": true, "office.com": true,
	"amazon.com": true, "amazon.in": true, "ebay.com": true,
	"paypal.com": true, "stripe.com": true,
	"cloudflare.com": true, "nginx.com": true,
	"openai.com": true, "chatgpt.com": true,
}

func init() {
	for _, service := range ExtraAudioServices {
		extraAudioSet[service] = true
	}
	for _, service := range ExtraVideoServices {
		extraVideoSet[service] = true
	}

	YTDLPServices = append(append([]string{}, YTDLPServices...), ExtraVideoServices...)
	YTDLPServices = append(YTDLPServices, ExtraAudioServices...)
	YTDLPServices = append(YTDLPServices, ServiceGeneric)

	Services = append(append([]string{}, Services...), ExtraVideoServices...)
	Services = append(Services, ExtraAudioServices...)
	Services = append(Services, ServiceGeneric)

	servicePatterns = append(servicePatterns, extraPatterns...)
}

// IsExtraAudio reports whether the platform prefers an audio file.
func IsExtraAudio(service string) bool {
	return extraAudioSet[service]
}

// IsExtraVideo reports whether the platform prefers a video or a photo.
func IsExtraVideo(service string) bool {
	return extraVideoSet[service]
}

// IsYTDLPService reports whether yt-dlp is the resolver of the platform.
func IsYTDLPService(service string) bool {
	if service == ServiceGeneric {
		return true
	}
	for _, item := range YTDLPServices {
		if item == service {
			return true
		}
	}
	return extraAudioSet[service] || extraVideoSet[service]
}

// ExtraServiceLabel returns the display name of an extra platform.
func ExtraServiceLabel(service string) string {
	if label, ok := extraLabel[service]; ok {
		return label
	}
	return ""
}

// KeepURLQuery reports whether CanonicalizeURL must keep the query of a
// platform. YouTube already has its own rule; extras and the generic
// catch-all keep the query because many extractors read it.
func KeepURLQuery(service string) bool {
	return extraAudioSet[service] || extraVideoSet[service] || service == ServiceGeneric
}

// ExtractGenericLink returns a web link when no named platform matched.
func ExtractGenericLink(text string) (string, string, bool) {
	found := genericURLRe.FindStringIndex(text)
	if found == nil {
		return "", "", false
	}
	raw := strings.TrimRight(text[found[0]:found[1]], trailingPunctuation)
	parsed, err := url.Parse(raw)
	if err != nil {
		return "", "", false
	}
	host := strings.ToLower(parsed.Hostname())
	if host == "" || genericSkipHosts[host] {
		return "", "", false
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", "", false
	}
	if parsed.Path == "" || parsed.Path == "/" {
		return "", "", false
	}
	return ServiceGeneric, CanonicalizeURL(ServiceGeneric, raw), true
}
