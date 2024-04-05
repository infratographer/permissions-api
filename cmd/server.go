package cmd

import (
	"context"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"go.infratographer.com/x/crdbx"
	"go.infratographer.com/x/echojwtx"
	"go.infratographer.com/x/echox"
	"go.infratographer.com/x/events"
	"go.infratographer.com/x/otelx"
	"go.infratographer.com/x/versionx"
	"go.uber.org/zap"

	"go.infratographer.com/permissions-api/internal/api"
	"go.infratographer.com/permissions-api/internal/config"
	"go.infratographer.com/permissions-api/internal/iapl"
	"go.infratographer.com/permissions-api/internal/query"
	"go.infratographer.com/permissions-api/internal/spicedbx"
	"go.infratographer.com/permissions-api/internal/storage"
)

var (
	apiDefaultListen = "0.0.0.0:7602"
)

var serverCmd = &cobra.Command{
	Use:   "server",
	Short: "starts the permissions-api server",
	Run: func(cmd *cobra.Command, args []string) {
		serve(cmd.Context(), globalCfg)
	},
}

func init() {
	rootCmd.AddCommand(serverCmd)

	v := viper.GetViper()

	echox.MustViperFlags(v, serverCmd.Flags(), apiDefaultListen)
	otelx.MustViperFlags(v, serverCmd.Flags())
	echojwtx.MustViperFlags(v, serverCmd.Flags())
	events.MustViperFlags(v, serverCmd.Flags(), appName)
}

func serve(ctx context.Context, cfg *config.AppConfig) {
	err := otelx.InitTracer(cfg.Tracing, appName, logger)
	if err != nil {
		logger.Fatalw("unable to initialize tracing system", "error", err)
	}

	spiceClient, err := spicedbx.NewClient(cfg.SpiceDB, cfg.Tracing.Enabled)
	if err != nil {
		logger.Fatalw("unable to initialize spicedb client", "error", err)
	}

	eventsConn, err := events.NewConnection(cfg.Events.Config, events.WithLogger(logger))
	if err != nil {
		logger.Fatalw("failed to initialize events", "error", err)
	}

	kv, err := initializeKV(cfg.Events, eventsConn)
	if err != nil {
		logger.Fatalw("failed to initialize KV", "error", err)
	}

	db, err := crdbx.NewDB(cfg.CRDB, cfg.Tracing.Enabled)
	if err != nil {
		logger.Fatalw("unable to initialize permissions-api database", "error", err)
	}

	store := storage.New(db, storage.WithLogger(logger))

	var policy iapl.Policy

	if cfg.SpiceDB.PolicyDir != "" {
		policy, err = iapl.NewPolicyFromDirectory(cfg.SpiceDB.PolicyDir)
		if err != nil {
			logger.Fatalw("unable to load new policy from schema directory", "policy_dir", cfg.SpiceDB.PolicyDir, "error", err)
		}
	} else {
		logger.Warn("no spicedb policy defined, using default policy")

		policy = iapl.DefaultPolicy()
	}

	if err = policy.Validate(); err != nil {
		logger.Fatalw("invalid spicedb policy", "error", err)
	}

	engine, err := query.NewEngine("infratographer", spiceClient, kv, store, query.WithPolicy(policy))
	if err != nil {
		logger.Fatalw("error creating engine", "error", err)
	}

	srv, err := echox.NewServer(
		logger.Desugar(),
		echox.ConfigFromViper(viper.GetViper()),
		versionx.BuildDetails(),
	)
	if err != nil {
		logger.Fatal("failed to initialize new server", zap.Error(err))
	}

	r, err := api.NewRouter(cfg.OIDC, engine, api.WithLogger(logger))
	if err != nil {
		logger.Fatalw("unable to initialize router", "error", err)
	}

	srv.AddHandler(r)
	srv.AddReadinessCheck("spicedb", spicedbx.Healthcheck(spiceClient))
	srv.AddReadinessCheck("storage", store.HealthCheck)

	if err := srv.Run(); err != nil {
		logger.Fatal("failed to run server", zap.Error(err))
	}
}
