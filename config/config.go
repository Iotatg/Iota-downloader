// Package config loads and validates the Iota Coder runtime configuration from
// the process environment. Every knob keeps the same name that the original
// downloader bot used, so existing deployments can be migrated by only
// changing values.
package config

import (
	"fmt"
	"net"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

// Command is a single entry of the Telegram command menu.
type Command struct {
	Command     string
	Description string
}

// Config is the fully resolved runtime configuration.
type Config struct {
	BotToken      string
	DatabaseURL   string
	AdminID       int64
	AdminIDsExtra []int64

	MeasurementID string
	APISecret     string
	ChannelID     string

	OutputDir string
	LogDir    string

	CobaltAPIURL string
	CobaltAPIKey string

	IOTAAPIURL string
	IOTAAPIKey string

	// RiteshAPIURL and RiteshAPIKey configure the Ritesh api, the
	// multi platform resolver of the bot.
	RiteshAPIURL string
	RiteshAPIKey string

	InstagramSessionID   string
	InstagramCookiesFile string
	InstagramAppID       string

	SpotifyClientID     string
	SpotifyClientSecret string
	SpotifyMarket       string

	DisableColour bool

	PollingConcurrencyLimit int
	SessionConnectionLimit  int

	DBPoolSize    int
	DBMaxOverflow int
	DBPoolTimeout float64

	AntiFloodMessageLimit    int
	AntiFloodMessageWindow   float64
	AntiFloodCallbackLimit   int
	AntiFloodCallbackWindow  float64
	AntiFloodInlineLimit     int
	AntiFloodInlineWindow    float64
	AntiFloodGuestLimit      int
	AntiFloodGuestWindow     float64
	AntiFloodGlobalLimit     int
	AntiFloodGlobalWindow    float64
	AntiFloodCooldown        float64
	AntiFloodUserTTL         float64
	AntiFloodMaxTrackedUsers int

	RequestDedupeActiveTTL    float64
	RequestDedupeCompletedTTL float64
	RequestDedupeMaxEntries   int

	ResolutionCacheTTL float64
	ResolutionCacheMax int

	DownloadQueueMinWorkers          int
	DownloadQueueMaxWorkers          int
	DownloadQueueMaxSize             int
	DownloadQueuePerUserRateLimit    int
	DownloadQueuePerUserWindow       float64
	DownloadQueuePerUserMaxPending   int
	DownloadQueuePerUserPendingLimit float64
	DownloadQueueScaleCooldown       float64
	DownloadQueueIdleScaleDown       float64
	DownloadMaxWorkersCap            int

	BatchLinksMaxItems                int
	BatchLinksMinConcurrency          int
	BatchLinksMaxConcurrency          int
	BatchLinksParallelQueueDepthLimit int
	BatchLinksParallelActiveJobsLimit int

	// CollectionMaxItems is the number of the entries of a playlist, an
	// album or a board the bot expands.
	CollectionMaxItems int
	// CollectionPageSize is the number of the entries the menu of a
	// collection shows at once.
	CollectionPageSize int
	// CollectionCacheTTL is the time the menu of a collection stays alive.
	CollectionCacheTTL float64
	// CollectionCacheMax is the number of the collections the bot keeps.
	CollectionCacheMax int

	MaxFileSize int64

	// APIID is the identifier of the Telegram application of the bot, the
	// client speaks MTProto and needs the identifier of an application.
	APIID int32
	// APIHash is the hash of the Telegram application of the bot.
	APIHash string
	// Session keeps the MTProto session of the bot between two runs.
	Session string
	// MTProtoFileLimitMB is the size limit of a single upload over MTProto.
	//
	// The server refuses every file part at or beyond 2000 MB, a bigger file
	// has to be split into parts.
	MTProtoFileLimitMB int
	// SplitLargeFiles turns on the split of the files that are too big for
	// a single upload over MTProto.
	SplitLargeFiles bool

	YTDLPCookiesFile               string
	YTDLPCookiesFromBrowser        string
	YTDLPYoutubeCookiesFile        string
	YTDLPYoutubeCookiesFromBrowser string
	YTDLPYoutubePlayerClient       string
	YTDLPYoutubePOToken            string
	YTDLPYoutubeRemoteComponents   string
	YTDLPYoutubeJSRuntimes         string
	YTDLPYoutubeInfoTimeout        float64
	YTDLPYoutubeSleepRequests      float64
	YTDLPYoutubeSleepInterval      float64
	YTDLPYoutubeMaxSleepInterval   float64

	DownloadSubprocessThresholdMB int

	Commands []Command
}

// BotCommands is the command menu published to Telegram.
var BotCommands = []Command{
	{Command: "start", Description: "Get started"},
	{Command: "help", Description: "Supported sites and tips"},
	{Command: "settings", Description: "Settings"},
	{Command: "stats", Description: "Statistics"},
	{Command: "remove_keyboard", Description: "Hide the reply keyboard"},
}

// DefaultInstagramAppID is the public application identifier of the Instagram
// mobile application. It is required by the private media endpoints.
const DefaultInstagramAppID = "936619743392459"

// AdminIDs returns the list of administrator identifiers.
//
// The primary administrator comes from ADMIN_ID, ADMIN_IDS holds a comma
// separated list of further administrators.
func (c *Config) AdminIDs() []int64 {
	ids := make([]int64, 0, len(c.AdminIDsExtra)+1)
	if c.AdminID > 0 {
		ids = append(ids, c.AdminID)
	}
	for _, id := range c.AdminIDsExtra {
		if id > 0 {
			ids = append(ids, id)
		}
	}
	return ids
}

// Duration helpers translate the fractional second settings into durations.

func (c *Config) DBPoolTimeoutDuration() time.Duration { return seconds(c.DBPoolTimeout) }
func (c *Config) AntiFloodMessageWindowDuration() time.Duration {
	return seconds(c.AntiFloodMessageWindow)
}
func (c *Config) AntiFloodCallbackWindowDuration() time.Duration {
	return seconds(c.AntiFloodCallbackWindow)
}
func (c *Config) AntiFloodInlineWindowDuration() time.Duration {
	return seconds(c.AntiFloodInlineWindow)
}
func (c *Config) AntiFloodGuestWindowDuration() time.Duration {
	return seconds(c.AntiFloodGuestWindow)
}
func (c *Config) AntiFloodGlobalWindowDuration() time.Duration {
	return seconds(c.AntiFloodGlobalWindow)
}
func (c *Config) AntiFloodCooldownDuration() time.Duration { return seconds(c.AntiFloodCooldown) }
func (c *Config) AntiFloodUserTTLDuration() time.Duration  { return seconds(c.AntiFloodUserTTL) }
func (c *Config) RequestDedupeActiveTTLDuration() time.Duration {
	return seconds(c.RequestDedupeActiveTTL)
}
func (c *Config) RequestDedupeCompletedTTLDuration() time.Duration {
	return seconds(c.RequestDedupeCompletedTTL)
}
func (c *Config) ResolutionCacheTTLDuration() time.Duration {
	return seconds(c.ResolutionCacheTTL)
}

// CollectionCacheTTLDuration returns the time the menu of a collection stays
// alive.
func (c *Config) CollectionCacheTTLDuration() time.Duration {
	return seconds(c.CollectionCacheTTL)
}

func (c *Config) DownloadQueuePerUserWindowDuration() time.Duration {
	return seconds(c.DownloadQueuePerUserWindow)
}
func (c *Config) DownloadQueuePerUserPendingLimitDuration() time.Duration {
	return seconds(c.DownloadQueuePerUserPendingLimit)
}
func (c *Config) DownloadQueueScaleCooldownDuration() time.Duration {
	return seconds(c.DownloadQueueScaleCooldown)
}
func (c *Config) DownloadQueueIdleScaleDownDuration() time.Duration {
	return seconds(c.DownloadQueueIdleScaleDown)
}
func (c *Config) YTDLPYoutubeInfoTimeoutDuration() time.Duration {
	return seconds(c.YTDLPYoutubeInfoTimeout)
}

func seconds(value float64) time.Duration {
	return time.Duration(value * float64(time.Second))
}

// Load reads the configuration from the environment and validates it.
func Load() (*Config, error) {
	c := &Config{
		OutputDir:     "downloads",
		LogDir:        "logs",
		SpotifyMarket: "UA",
		Commands:      BotCommands,
	}

	var err error
	if c.BotToken, err = readEnv("BOT_TOKEN", true); err != nil {
		return nil, err
	}
	if c.DatabaseURL, err = readEnv("DATABASE_URL", true); err != nil {
		return nil, err
	}
	if c.AdminID, err = readInt64Env("ADMIN_ID", true, "admin_id"); err != nil {
		return nil, err
	}
	if c.AdminIDsExtra, err = readInt64ListEnv("ADMIN_IDS"); err != nil {
		return nil, err
	}

	c.MeasurementID, _ = readEnv("MEASUREMENT_ID", false)
	c.APISecret, _ = readEnv("API_SECRET", false)
	c.ChannelID, _ = readEnv("CHANNEL_ID", false)
	c.CobaltAPIKey, _ = readEnv("COBALT_API_KEY", false)
	c.InstagramSessionID, _ = readEnv("INSTAGRAM_SESSIONID", false)
	c.InstagramCookiesFile, _ = readEnv("INSTAGRAM_COOKIES_FILE", false)
	c.InstagramAppID, _ = readEnv("INSTAGRAM_APP_ID", false)
	if c.InstagramAppID == "" {
		c.InstagramAppID = DefaultInstagramAppID
	}
	c.SpotifyClientID, _ = readEnv("SPOTIFY_CLIENT_ID", false)
	c.SpotifyClientSecret, _ = readEnv("SPOTIFY_CLIENT_SECRET", false)
	if market, err := readEnv("SPOTIFY_MARKET", false); err == nil && market != "" {
		c.SpotifyMarket = market
	}

	if c.CobaltAPIURL, err = readEnv("COBALT_API_URL", false); err == nil && c.CobaltAPIURL != "" {
		if os.Getenv("COBALT_API_URL_DANGER_ACCEPT_INSECURE") == "" {
			if c.CobaltAPIURL, err = validateCobaltURL(c.CobaltAPIURL); err != nil {
				return nil, err
			}
		}
	}

	if c.IOTAAPIURL, err = readEnv("IOTA_API_URL", false); err == nil && c.IOTAAPIURL != "" {
		if c.IOTAAPIURL, err = validateHTTPURL(c.IOTAAPIURL, "IOTA_API_URL"); err != nil {
			return nil, err
		}
	}
	c.IOTAAPIKey, _ = readEnv("IOTA_API_KEY", false)

	if c.RiteshAPIURL, err = readEnv("RITESH_API_URL", false); err == nil && c.RiteshAPIURL != "" {
		if c.RiteshAPIURL, err = validateHTTPURL(c.RiteshAPIURL, "RITESH_API_URL"); err != nil {
			return nil, err
		}
	}
	c.RiteshAPIKey, _ = readEnv("RITESH_API_KEY", false)

	if dir, envErr := readEnv("OUTPUT_DIR", false); envErr == nil && dir != "" {
		c.OutputDir = dir
	}
	if dir, envErr := readEnv("LOG_DIR", false); envErr == nil && dir != "" {
		c.LogDir = dir
	}

	ints := []struct {
		name     string
		target   *int
		fallback int
	}{
		{"BOT_POLLING_TASKS_CONCURRENCY_LIMIT", &c.PollingConcurrencyLimit, 256},
		{"BOT_SESSION_CONNECTION_LIMIT", &c.SessionConnectionLimit, 400},
		{"DB_POOL_SIZE", &c.DBPoolSize, 5},
		{"DB_MAX_OVERFLOW", &c.DBMaxOverflow, 10},
		{"ANTIFLOOD_MESSAGE_LIMIT", &c.AntiFloodMessageLimit, 4},
		{"ANTIFLOOD_CALLBACK_LIMIT", &c.AntiFloodCallbackLimit, 6},
		{"ANTIFLOOD_INLINE_LIMIT", &c.AntiFloodInlineLimit, 4},
		{"ANTIFLOOD_GUEST_LIMIT", &c.AntiFloodGuestLimit, 4},
		{"ANTIFLOOD_GLOBAL_LIMIT", &c.AntiFloodGlobalLimit, 8},
		{"ANTIFLOOD_MAX_TRACKED_USERS", &c.AntiFloodMaxTrackedUsers, 50000},
		{"REQUEST_DEDUPE_MAX_ENTRIES", &c.RequestDedupeMaxEntries, 50000},
		{"RESOLUTION_CACHE_MAX_ENTRIES", &c.ResolutionCacheMax, 500},
		{"DOWNLOAD_QUEUE_MIN_WORKERS", &c.DownloadQueueMinWorkers, 3},
		{"DOWNLOAD_QUEUE_MAX_WORKERS", &c.DownloadQueueMaxWorkers, 7},
		{"DOWNLOAD_QUEUE_MAX_SIZE", &c.DownloadQueueMaxSize, 250},
		{"DOWNLOAD_QUEUE_PER_USER_RATE_LIMIT", &c.DownloadQueuePerUserRateLimit, 4},
		{"DOWNLOAD_QUEUE_PER_USER_MAX_PENDING", &c.DownloadQueuePerUserMaxPending, 3},
		{"DOWNLOAD_MAX_WORKERS_CAP", &c.DownloadMaxWorkersCap, 6},
		{"BATCH_LINKS_MAX_ITEMS", &c.BatchLinksMaxItems, 6},
		{"COLLECTION_MAX_ITEMS", &c.CollectionMaxItems, 50},
		{"COLLECTION_PAGE_SIZE", &c.CollectionPageSize, 10},
		{"COLLECTION_CACHE_MAX_ENTRIES", &c.CollectionCacheMax, 50},
		{"BATCH_LINKS_MIN_CONCURRENCY", &c.BatchLinksMinConcurrency, 1},
		{"BATCH_LINKS_MAX_CONCURRENCY", &c.BatchLinksMaxConcurrency, 4},
		{"BATCH_LINKS_PARALLEL_QUEUE_DEPTH_THRESHOLD", &c.BatchLinksParallelQueueDepthLimit, 4},
		{"BATCH_LINKS_PARALLEL_ACTIVE_JOBS_THRESHOLD", &c.BatchLinksParallelActiveJobsLimit, 5},
		{"DOWNLOAD_SUBPROCESS_THRESHOLD_MB", &c.DownloadSubprocessThresholdMB, 96},
	}
	for _, item := range ints {
		value, readErr := readIntEnv(item.name, false)
		if readErr != nil {
			return nil, readErr
		}
		if value == 0 {
			*item.target = item.fallback
			continue
		}
		*item.target = value
	}

	floats := []struct {
		name     string
		target   *float64
		fallback float64
	}{
		{"DB_POOL_TIMEOUT", &c.DBPoolTimeout, 15},
		{"ANTIFLOOD_MESSAGE_WINDOW_SECONDS", &c.AntiFloodMessageWindow, 2},
		{"ANTIFLOOD_CALLBACK_WINDOW_SECONDS", &c.AntiFloodCallbackWindow, 2},
		{"ANTIFLOOD_INLINE_WINDOW_SECONDS", &c.AntiFloodInlineWindow, 3},
		{"ANTIFLOOD_GUEST_WINDOW_SECONDS", &c.AntiFloodGuestWindow, 3},
		{"ANTIFLOOD_GLOBAL_WINDOW_SECONDS", &c.AntiFloodGlobalWindow, 3},
		{"ANTIFLOOD_COOLDOWN_SECONDS", &c.AntiFloodCooldown, 6},
		{"ANTIFLOOD_USER_TTL_SECONDS", &c.AntiFloodUserTTL, 180},
		{"REQUEST_DEDUPE_ACTIVE_TTL_SECONDS", &c.RequestDedupeActiveTTL, 900},
		{"REQUEST_DEDUPE_COMPLETED_TTL_SECONDS", &c.RequestDedupeCompletedTTL, 12},
		{"RESOLUTION_CACHE_TTL_SECONDS", &c.ResolutionCacheTTL, 300},
		{"COLLECTION_CACHE_TTL_SECONDS", &c.CollectionCacheTTL, 1800},
		{"DOWNLOAD_QUEUE_PER_USER_WINDOW_SECONDS", &c.DownloadQueuePerUserWindow, 10},
		{"DOWNLOAD_QUEUE_SCALE_COOLDOWN_SECONDS", &c.DownloadQueueScaleCooldown, 8},
		{"DOWNLOAD_QUEUE_IDLE_SCALE_DOWN_SECONDS", &c.DownloadQueueIdleScaleDown, 35},
		{"YTDLP_YOUTUBE_SLEEP_REQUESTS_SECONDS", &c.YTDLPYoutubeSleepRequests, 0},
		{"YTDLP_YOUTUBE_SLEEP_INTERVAL_SECONDS", &c.YTDLPYoutubeSleepInterval, 0},
		{"YTDLP_YOUTUBE_MAX_SLEEP_INTERVAL_SECONDS", &c.YTDLPYoutubeMaxSleepInterval, 0},
	}
	for _, item := range floats {
		value, readErr := readFloatEnv(item.name, false)
		if readErr != nil {
			return nil, readErr
		}
		if value == 0 && item.fallback != 0 {
			*item.target = item.fallback
			continue
		}
		*item.target = value
	}

	if c.DownloadQueuePerUserPendingLimit, err = readFloatEnv("DOWNLOAD_QUEUE_PER_USER_PENDING_TIMEOUT_SECONDS", false); err != nil {
		return nil, err
	}
	if c.DownloadQueuePerUserPendingLimit == 0 {
		c.DownloadQueuePerUserPendingLimit = 20
	}
	if c.YTDLPYoutubeInfoTimeout, err = readFloatEnv("YTDLP_YOUTUBE_INFO_TIMEOUT_SECONDS", false); err != nil {
		return nil, err
	}
	if c.YTDLPYoutubeInfoTimeout == 0 {
		c.YTDLPYoutubeInfoTimeout = 45
	}

	maxFileSize, err := readInt64Env("MAX_FILE_SIZE", false)
	if err != nil {
		return nil, err
	}
	if maxFileSize <= 0 {
		maxFileSize = int64(2000) * 1024 * 1024
	}
	c.MaxFileSize = maxFileSize

	c.YTDLPYoutubeCookiesFile, _ = readEnv("YTDLP_YOUTUBE_COOKIES_FILE", false)
	if c.YTDLPYoutubeCookiesFile == "" {
		c.YTDLPYoutubeCookiesFile = "cookies/youtube.txt"
	}
	c.YTDLPYoutubeCookiesFromBrowser, _ = readEnv("YTDLP_YOUTUBE_COOKIES_FROM_BROWSER", false)

	// The cookies of youtube are also used by the other platforms, a
	// separate file serves the platforms that ask for a session.
	c.YTDLPCookiesFile, _ = readEnv("YTDLP_COOKIES_FILE", false)
	if c.YTDLPCookiesFile == "" {
		c.YTDLPCookiesFile = c.YTDLPYoutubeCookiesFile
	}
	c.YTDLPCookiesFromBrowser, _ = readEnv("YTDLP_COOKIES_FROM_BROWSER", false)
	if c.YTDLPCookiesFromBrowser == "" {
		c.YTDLPCookiesFromBrowser = c.YTDLPYoutubeCookiesFromBrowser
	}

	c.SplitLargeFiles = readBoolEnv("SPLIT_LARGE_FILES", true)

	apiID, err := readIntEnv("API_ID", false, "MTPROTO_API_ID")
	if err != nil {
		return nil, err
	}
	c.APIID = int32(apiID)
	if c.APIHash, err = readEnv("API_HASH", true, "MTPROTO_API_HASH"); err != nil {
		return nil, err
	}
	if c.Session, err = readEnv("MTPROTO_SESSION", false); err != nil {
		return nil, err
	}
	if c.Session == "" {
		c.Session = "session/iota"
	}

	mtprotoLimit, err := readIntEnv("MTPROTO_FILE_LIMIT_MB", false)
	if err != nil {
		return nil, err
	}
	if mtprotoLimit <= 0 {
		mtprotoLimit = 2000
	}
	c.MTProtoFileLimitMB = mtprotoLimit

	c.YTDLPYoutubePlayerClient, _ = readEnv("YTDLP_YOUTUBE_PLAYER_CLIENT", false)
	c.YTDLPYoutubePOToken, _ = readEnv("YTDLP_YOUTUBE_PO_TOKEN", false)
	c.YTDLPYoutubeRemoteComponents, _ = readEnv("YTDLP_YOUTUBE_REMOTE_COMPONENTS", false)
	c.YTDLPYoutubeJSRuntimes, _ = readEnv("YTDLP_YOUTUBE_JS_RUNTIMES", false)

	c.DisableColour = readBoolEnv("DISABLE_COLOUR", false)

	return c, nil
}

// readBoolEnv reads a boolean environment variable.
func readBoolEnv(name string, fallback bool) bool {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func readEnv(name string, required bool, aliases ...string) (string, error) {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		for _, alias := range aliases {
			value = strings.TrimSpace(os.Getenv(alias))
			if value != "" {
				break
			}
		}
	}
	if value == "" && required {
		return "", fmt.Errorf("required environment variable %s is not set", name)
	}
	return value, nil
}

func readIntEnv(name string, required bool, aliases ...string) (int, error) {
	raw, err := readEnv(name, required, aliases...)
	if err != nil {
		return 0, err
	}
	if raw == "" {
		return 0, nil
	}
	value, convErr := strconv.Atoi(raw)
	if convErr != nil {
		return 0, fmt.Errorf("environment variable %s must be an integer", name)
	}
	return value, nil
}

func readInt64Env(name string, required bool, aliases ...string) (int64, error) {
	raw, err := readEnv(name, required, aliases...)
	if err != nil {
		return 0, err
	}
	if raw == "" {
		return 0, nil
	}
	value, convErr := strconv.ParseInt(raw, 10, 64)
	if convErr != nil {
		return 0, fmt.Errorf("environment variable %s must be an integer", name)
	}
	return value, nil
}

// readInt64ListEnv reads a comma separated list of integers.
func readInt64ListEnv(name string) ([]int64, error) {
	raw, err := readEnv(name, false)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(raw) == "" {
		return nil, nil
	}

	parts := strings.Split(raw, ",")
	values := make([]int64, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		value, convErr := strconv.ParseInt(part, 10, 64)
		if convErr != nil {
			return nil, fmt.Errorf("environment variable %s must be a comma separated list of integers", name)
		}
		values = append(values, value)
	}
	return values, nil
}

func readFloatEnv(name string, required bool, aliases ...string) (float64, error) {
	raw, err := readEnv(name, required, aliases...)
	if err != nil {
		return 0, err
	}
	if raw == "" {
		return 0, nil
	}
	value, convErr := strconv.ParseFloat(raw, 64)
	if convErr != nil {
		return 0, fmt.Errorf("environment variable %s must be a float", name)
	}
	return value, nil
}

func validateHTTPURL(value, name string) (string, error) {
	if value == "" {
		return value, nil
	}
	parsed, err := url.Parse(value)
	if err != nil {
		return "", fmt.Errorf("invalid URL for %s: %w", name, err)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", fmt.Errorf("invalid scheme for %s: %s (must be http or https)", name, parsed.Scheme)
	}
	if strings.Trim(parsed.Hostname(), "[]") == "" {
		return "", fmt.Errorf("no host in %s: %s", name, value)
	}
	return value, nil
}

func validateCobaltURL(value string) (string, error) {
	parsed, err := url.Parse(value)
	if err != nil {
		return "", fmt.Errorf("invalid URL for COBALT_API_URL: %w", err)
	}
	if parsed.Scheme != "https" {
		return "", fmt.Errorf("HTTPS is required for COBALT_API_URL, found %q; use https:// or set COBALT_API_URL_DANGER_ACCEPT_INSECURE=true to bypass", parsed.Scheme)
	}
	host := strings.Trim(parsed.Hostname(), "[]")
	if host == "" {
		return "", fmt.Errorf("no host in COBALT_API_URL: %s", value)
	}
	if ip := net.ParseIP(host); ip != nil && isPrivateIP(ip) {
		return "", fmt.Errorf("host %s in COBALT_API_URL resolves to a private/internal IP range", host)
	}
	return value, nil
}

func isPrivateIP(ip net.IP) bool {
	private := []string{"10.0.0.0/8", "172.16.0.0/12", "192.168.0.0/16", "169.254.0.0/16", "127.0.0.0/8", "::1/128", "fc00::/7"}
	for _, block := range private {
		_, network, err := net.ParseCIDR(block)
		if err != nil {
			continue
		}
		if network.Contains(ip) {
			return true
		}
	}
	return false
}
