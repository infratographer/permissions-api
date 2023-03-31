package cmd

import (
	"context"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"go.hollow.sh/toolbox/ginjwt"
	"go.infratographer.com/x/ginx"
	"go.infratographer.com/x/otelx"
	"go.infratographer.com/x/versionx"

	"go.infratographer.com/permissions-api/internal/api"
	"go.infratographer.com/permissions-api/internal/config"
	"go.infratographer.com/permissions-api/internal/query"
	"go.infratographer.com/permissions-api/internal/spicedbx"
)

var (
	apiDefaultListen = "0.0.0.0:7602"
)

var serverCmd = &cobra.Command{
	Use:   "server",
	Short: "starts the permission api server",
	Run: func(cmd *cobra.Command, args []string) {
		serve(cmd.Context(), globalCfg)
	},
}

func init() {
	rootCmd.AddCommand(serverCmd)

	v := viper.GetViper()

	ginx.MustViperFlags(v, serverCmd.Flags(), apiDefaultListen)
	otelx.MustViperFlags(v, serverCmd.Flags())
	ginjwt.RegisterViperOIDCFlags(v, serverCmd)
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

	engine := query.NewEngine("infratographer", spiceClient)

	s := ginx.NewServer(logger.Desugar(), cfg.Server, versionx.BuildDetails())

	r, err := api.NewRouter(cfg.OIDC, engine, logger)
	if err != nil {
		logger.Fatalw("unable to initialize router", "error", err)
	}

	s = s.AddHandler(r).
		AddReadinessCheck("spicedb", spicedbx.Healthcheck(spiceClient))

	s.Run()
}
