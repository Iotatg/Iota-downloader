// Package app wires every component of the Iota Coder bot together and runs
// the update loop.
package app

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/Iotatg/iota-downloader/config"
	"github.com/Iotatg/iota-downloader/internal/download"
	"github.com/Iotatg/iota-downloader/internal/handlers"
	"github.com/Iotatg/iota-downloader/internal/keyboards"
	"github.com/Iotatg/iota-downloader/internal/logger"
	"github.com/Iotatg/iota-downloader/internal/media"
	"github.com/Iotatg/iota-downloader/internal/platforms"
	"github.com/Iotatg/iota-downloader/internal/runtime"
	"github.com/Iotatg/iota-downloader/internal/services"
	"github.com/Iotatg/iota-downloader/internal/storage"
	"github.com/Iotatg/iota-downloader/internal/tg"
)

// App holds the running components of the bot.
type App struct {
	Config  *config.Config
	Log     *logger.Logger
	DB      *storage.DB
	Client  *tg.Client
	Bot     *tg.User
	Queue   *download.Queue
	Stats   *runtime.Stats
	Flow    *media.Flow
	Handler *handlers.Handler

	analytics *handlers.AnalyticsRecorder
	cancel    context.CancelFunc
	workers   sync.WaitGroup
}

// New builds the application from the environment configuration.
func New(ctx context.Context) (*App, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, err
	}

	log, err := logger.New(logger.Options{Dir: cfg.LogDir, Debug: strings.EqualFold(os.Getenv("DEBUG"), "true")})
	if err != nil {
		return nil, err
	}

	keyboards.SetColourEnabled(!cfg.DisableColour)

	db, err := storage.Open(ctx, storage.Options{
		DSN:         cfg.DatabaseURL,
		PoolSize:    cfg.DBPoolSize,
		MaxOverflow: cfg.DBMaxOverflow,
		PoolTimeout: cfg.DBPoolTimeoutDuration(),
		Logger:      log,
	})
	if err != nil {
		log.Error("database is not available: %v", err)
		return nil, err
	}

	if err := db.Init(ctx); err != nil {
		log.Error("database migration failed: %v", err)
		return nil, err
	}

	client, err := tg.New(cfg.BotToken, cfg.APIID, cfg.APIHash, tg.WithSession(cfg.Session))
	if err != nil {
		log.Error("connect the bot over mtproto failed: %v", err)
		return nil, err
	}

	bot, err := client.GetMe(ctx)
	if err != nil {
		log.Error("read the bot account failed: %v", err)
		return nil, err
	}

	registry, err := buildServices(cfg, log)
	if err != nil {
		log.Error("build the platform services failed: %v", err)
		return nil, err
	}

	stats := runtime.NewStats()
	dedupe := runtime.NewDedupe(
		cfg.RequestDedupeActiveTTLDuration(),
		cfg.RequestDedupeCompletedTTLDuration(),
		cfg.RequestDedupeMaxEntries,
	)
	limiter := runtime.NewRateLimiter(
		cfg.AntiFloodMessageLimit,
		cfg.AntiFloodMessageWindowDuration(),
		cfg.AntiFloodMaxTrackedUsers,
	)
	state := runtime.NewStateStore(30 * time.Minute)

	downloader := download.NewDownloader(cfg.OutputDir, download.DefaultConfig(), log)
	queue := download.NewQueue(download.QueueOptions{
		MinWorkers:            cfg.DownloadQueueMinWorkers,
		MaxWorkers:            cfg.DownloadQueueMaxWorkers,
		MaxSize:               cfg.DownloadQueueMaxSize,
		PerUserRateLimit:      cfg.DownloadQueuePerUserRateLimit,
		PerUserWindow:         cfg.DownloadQueuePerUserWindowDuration(),
		PerUserMaxPending:     cfg.DownloadQueuePerUserMaxPending,
		PerUserPendingTimeout: cfg.DownloadQueuePerUserPendingLimitDuration(),
		ScaleCooldown:         cfg.DownloadQueueScaleCooldownDuration(),
		IdleScaleDown:         cfg.DownloadQueueIdleScaleDownDuration(),
		MaxWorkersCap:         cfg.DownloadMaxWorkersCap,
	}, log)

	flow := &media.Flow{
		DB:          db,
		Stats:       stats,
		Dedupe:      dedupe,
		Log:         log,
		MaxFileSize: cfg.MaxFileSize,
	}

	analytics := handlers.NewAnalyticsRecorder(db, log, 30*time.Second)

	handler := handlers.New(handlers.Deps{
		Config:     cfg,
		Log:        log,
		DB:         db,
		TG:         client,
		Bot:        bot,
		Flow:       flow,
		Queue:      queue,
		Downloader: downloader,
		Stats:      stats,
		Dedupe:     dedupe,
		Limiter:    limiter,
		State:      state,
		Services:   registry,
		Analytics:  analytics,
	})

	return &App{
		Config:    cfg,
		Log:       log,
		DB:        db,
		Client:    client,
		Bot:       bot,
		Queue:     queue,
		Stats:     stats,
		Flow:      flow,
		Handler:   handler,
		analytics: analytics,
	}, nil
}

// buildServices wires the platform resolvers.
func buildServices(cfg *config.Config, log *logger.Logger) (*services.Registry, error) {
	cobalt := platforms.NewCobaltClient(cfg.CobaltAPIURL, cfg.CobaltAPIKey, log)

	ytdlp := platforms.NewYTDLP(platforms.YTDLPOptions{
		CookiesFile:        cfg.YTDLPCookiesFile,
		CookiesFromBrowser: cfg.YTDLPCookiesFromBrowser,
		PlayerClient:       cfg.YTDLPYoutubePlayerClient,
		POToken:            cfg.YTDLPYoutubePOToken,
		RemoteComponents:   cfg.YTDLPYoutubeRemoteComponents,
		JSRuntimes:         cfg.YTDLPYoutubeJSRuntimes,
		InfoTimeout:        cfg.YTDLPYoutubeInfoTimeoutDuration(),
		SleepRequests:      cfg.YTDLPYoutubeSleepRequests,
		SleepInterval:      cfg.YTDLPYoutubeSleepInterval,
		MaxSleepInterval:   cfg.YTDLPYoutubeMaxSleepInterval,
	}, log)

	instagram := platforms.NewInstagramService(cobalt, log)
	instagram.SessionID = cfg.InstagramSessionID
	instagram.CookiesFile = cfg.InstagramCookiesFile
	instagram.AppID = cfg.InstagramAppID
	instagram.YTDLP = ytdlp

	return &services.Registry{
		YouTube:     platforms.NewYouTubeService(ytdlp, cfg.OutputDir, log),
		TikTok:      platforms.NewTikTokService(log),
		Twitter:     platforms.NewTwitterService(log),
		Threads:     platforms.NewThreadsService(log, cfg.InstagramSessionID),
		Instagram:   instagram,
		Pinterest:   platforms.NewPinterestService(cobalt, log),
		SoundCloud:  platforms.NewSoundCloudService(cobalt, log),
		Spotify:     platforms.NewSpotifyService(cfg.SpotifyClientID, cfg.SpotifyClientSecret, cfg.SpotifyMarket, log),
		Mastodon:    platforms.NewMastodonService(log),
		IotaAPI:     platforms.NewIotaAPIService(cfg.IOTAAPIURL, cfg.IOTAAPIKey, log),
		Ritesh:      platforms.NewRiteshService(cfg.RiteshAPIURL, cfg.RiteshAPIKey, log),
		Deezer:      platforms.NewDeezerService(log),
		YTDLP:       ytdlp,
		Cache:       services.NewResolutionCache(cfg.ResolutionCacheTTLDuration(), cfg.ResolutionCacheMax),
		Collections: services.NewCollectionCache(cfg.CollectionCacheTTLDuration(), cfg.CollectionCacheMax),
		Log:         log,
	}, nil
}

// Run starts the bot and blocks until the context is cancelled.
func (a *App) Run(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)
	a.cancel = cancel
	defer cancel()

	if err := a.publishCommands(ctx); err != nil {
		a.Log.Warn("publish the command menu failed: %v", err)
	}

	a.Log.Info("%s started as @%s", messages_BrandName, a.Bot.Username)

	return a.Client.Run(ctx, func(update tg.Update) {
		a.workers.Add(1)
		go func(current tg.Update) {
			defer a.workers.Done()
			a.handleUpdate(ctx, current)
		}(update)
	})
}

// handleUpdate processes a single update.
func (a *App) handleUpdate(ctx context.Context, update tg.Update) {
	defer func() {
		if recovered := recover(); recovered != nil {
			a.Log.Error("panic while handling an update: %v", recovered)
		}
	}()
	a.Handler.Handle(ctx, update)
}

// publishCommands publishes the command menu of the bot.
func (a *App) publishCommands(ctx context.Context) error {
	commands := make([]tg.BotCommand, 0, len(a.Config.Commands))
	for _, command := range a.Config.Commands {
		commands = append(commands, tg.BotCommand{Command: command.Command, Description: command.Description})
	}
	if len(commands) == 0 {
		return nil
	}
	return a.Client.SetMyCommands(ctx, commands)
}

// Close releases every resource of the application.
func (a *App) Close() {
	if a.cancel != nil {
		a.cancel()
	}
	if a.analytics != nil {
		a.analytics.Close()
	}
	if a.Queue != nil {
		a.Queue.Close()
	}
	if a.DB != nil {
		if err := a.DB.Close(); err != nil {
			a.Log.Error("close the database failed: %v", err)
		}
	}
}

// Wait blocks until every worker finished.
func (a *App) Wait() { a.workers.Wait() }

// messages_BrandName keeps the brand name in one place.
const messages_BrandName = "Iota Coder"

// RunUntilSignal runs the bot until the process receives an interrupt.
func RunUntilSignal(ctx context.Context) error {
	application, err := New(ctx)
	if err != nil {
		return err
	}
	defer application.Close()

	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt, syscall.SIGTERM)

	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	done := make(chan error, 1)
	go func() { done <- application.Run(runCtx) }()

	select {
	case err := <-done:
		return err
	case sig := <-signals:
		application.Log.Info("received the signal %s, shutting down", sig)
		cancel()
		application.Wait()
		return nil
	}
}

// ErrNotConfigured is returned when the environment is incomplete.
var ErrNotConfigured = errors.New("the bot is not configured correctly")

// ensure the error helper stays referenced.
var _ = fmt.Sprintf
