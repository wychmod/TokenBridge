package app

import (
	"context"
	"os"

	"github.com/rs/zerolog"
	"gorm.io/gorm"

	"localgateway/internal/admin"
	"localgateway/internal/auth"
	"localgateway/internal/config"
	"localgateway/internal/paths"
	"localgateway/internal/pricing"
	"localgateway/internal/provider"
	"localgateway/internal/requestlog"
	"localgateway/internal/routing"
	"localgateway/internal/server"
	"localgateway/internal/settings"
	"localgateway/internal/storage"
	"localgateway/internal/usage"
)

type Application struct {
	Config      config.Config
	Router      *server.Router
	Logger      zerolog.Logger
	DB          *gorm.DB
	Providers   *provider.Service
	Keys        *auth.Service
	Routing     *routing.Service
	Usage       *usage.Service
	Pricing     *pricing.Service
	Settings    *settings.Service
	Admin       *admin.Service
	RequestLogs *requestlog.Service
}

func New() (*Application, error) {
	appPaths, err := paths.Resolve()
	if err != nil {
		return nil, err
	}

	cfgPath := os.Getenv("LG_CONFIG")
	if cfgPath == "" {
		cfgPath, err = paths.EnsureUserConfig(appPaths)
		if err != nil {
			return nil, err
		}
	}

	cfg, err := config.Load(cfgPath)
	if err != nil {
		return nil, err
	}
	cfg.Database.Path = paths.ResolveUserDataPath(appPaths, cfg.Database.Path, "data/localgateway.db")
	cfg.Security.EncryptionKeyFile = paths.ResolveUserDataPath(appPaths, cfg.Security.EncryptionKeyFile, ".secret")

	if err := paths.MigrateLegacyDatabase(cfg.Database.Path); err != nil {
		return nil, err
	}

	logger := zerolog.New(os.Stdout).With().Timestamp().Str("service", "localgateway").Logger()
	db, err := storage.OpenDatabase(cfg.Database)
	if err != nil {
		return nil, err
	}

	providerService := provider.NewService(db)
	keyService := auth.NewService(db)
	usageService := usage.NewService(db)
	requestLogService := requestlog.NewService(db)
	routingService := routing.NewService(db, providerService, cfg.Routing.DefaultStrategy)
	settingsService := settings.NewService(db)
	pricingService := pricing.NewService(db, logger)
	adminService := admin.NewService(providerService, keyService, usageService, routingService, settingsService, requestLogService)

	application := &Application{
		Config:      cfg,
		Logger:      logger,
		DB:          db,
		Providers:   providerService,
		Keys:        keyService,
		Routing:     routingService,
		Usage:       usageService,
		Pricing:     pricingService,
		Settings:    settingsService,
		Admin:       adminService,
		RequestLogs: requestLogService,
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
		DB:          db,
	})

	return application, nil
}
