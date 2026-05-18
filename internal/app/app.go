package app

import (
	"context"
	"os"

	"github.com/rs/zerolog"
	"gorm.io/gorm"

	"tokenbridge/internal/admin"
	"tokenbridge/internal/aitoolusage"
	"tokenbridge/internal/auth"
	"tokenbridge/internal/config"
	"tokenbridge/internal/paths"
	"tokenbridge/internal/pricing"
	"tokenbridge/internal/provider"
	"tokenbridge/internal/requestlog"
	"tokenbridge/internal/routing"
	"tokenbridge/internal/server"
	"tokenbridge/internal/settings"
	"tokenbridge/internal/storage"
	"tokenbridge/internal/usage"
)

type Application struct {
	Config      config.Config
	Router      *server.Router
	Logger      zerolog.Logger
	CloseLogs   func() error
	DB          *gorm.DB
	Providers   *provider.Service
	Keys        *auth.Service
	Routing     *routing.Service
	Usage       *usage.Service
	Pricing     *pricing.Service
	Settings    *settings.Service
	Admin       *admin.Service
	RequestLogs *requestlog.Service
	AIToolUsage *aitoolusage.Service
}

func New() (*Application, error) {
	appPaths, err := paths.Resolve()
	if err != nil {
		return nil, err
	}
	if err := paths.EnsureUserDirs(appPaths); err != nil {
		return nil, err
	}

	logger, closeLogs, err := newRuntimeLogger(appPaths.LogDir, os.Stdout)
	if err != nil {
		return nil, err
	}
	fail := func(err error) (*Application, error) {
		logger.Error().Err(err).Msg("application startup failed")
		_ = closeLogs()
		return nil, err
	}

	cfgPath := os.Getenv("TB_CONFIG")
	if cfgPath == "" {
		cfgPath, err = paths.EnsureUserConfig(appPaths)
		if err != nil {
			return fail(err)
		}
	}

	cfg, err := config.Load(cfgPath)
	if err != nil {
		return fail(err)
	}
	cfg.Database.Path = paths.ResolveUserDataPath(appPaths, cfg.Database.Path, "data/tokenbridge.db")
	cfg.Security.EncryptionKeyFile = paths.ResolveUserDataPath(appPaths, cfg.Security.EncryptionKeyFile, ".secret")

	if err := paths.MigrateLegacyDatabase(cfg.Database.Path); err != nil {
		return fail(err)
	}

	db, err := storage.OpenDatabase(cfg.Database)
	if err != nil {
		return fail(err)
	}

	providerService := provider.NewService(db)
	keyService := auth.NewService(db)
	usageService := usage.NewService(db)
	requestLogService := requestlog.NewService(db)
	routingService := routing.NewService(db, providerService, cfg.Routing.DefaultStrategy)
	settingsService := settings.NewService(db)
	pricingService := pricing.NewService(db, logger)
	aiToolUsageService := aitoolusage.NewService(db, pricingService, logger)
	adminService := admin.NewService(providerService, keyService, usageService, pricingService, routingService, settingsService, requestLogService)

	application := &Application{
		Config:      cfg,
		Logger:      logger,
		CloseLogs:   closeLogs,
		DB:          db,
		Providers:   providerService,
		Keys:        keyService,
		Routing:     routingService,
		Usage:       usageService,
		Pricing:     pricingService,
		Settings:    settingsService,
		Admin:       adminService,
		RequestLogs: requestLogService,
		AIToolUsage: aiToolUsageService,
	}

	// Ensure first startup has local pricing data from the embedded snapshot, then update remotely in the background.
	pricingCtx := context.Background()
	if count, err := pricingService.EnsureLocalCache(pricingCtx); err != nil {
		logger.Warn().Err(err).Msg("pricing: failed to initialize embedded pricing cache")
	} else {
		logger.Info().Int("models", count).Msg("pricing: local pricing cache ready")
	}
	go func() {
		pricingService.SyncBestEffort(context.Background())
	}()
	go func() {
		if result, err := aiToolUsageService.Scan(context.Background()); err != nil {
			logger.Warn().Err(err).Msg("ai tool usage: startup scan failed")
		} else {
			logger.Info().Int("files", result.FilesScanned).Int64("created", result.RecordsCreated).Msg("ai tool usage: startup scan complete")
		}
	}()

	application.Router = server.NewRouter(server.Dependencies{
		Config:      cfg,
		Logger:      logger,
		Providers:   providerService,
		Keys:        keyService,
		Routing:     routingService,
		Usage:       usageService,
		Pricing:     pricingService,
		Settings:    settingsService,
		Admin:       adminService,
		RequestLogs: requestLogService,
		AIToolUsage: aiToolUsageService,
		DB:          db,
	})

	return application, nil
}
