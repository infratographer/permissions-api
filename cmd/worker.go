package cmd

import (
	"context"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"go.infratographer.com/x/otelx"

	"go.infratographer.com/permissions-api/internal/config"
	"go.infratographer.com/permissions-api/internal/pubsub"
	"go.infratographer.com/permissions-api/internal/query"
	"go.infratographer.com/permissions-api/internal/spicedbx"
)

var workerCmd = &cobra.Command{
	Use:   "worker",
	Short: "starts a permissions-api queue worker",
	Run: func(cmd *cobra.Command, args []string) {
		worker(cmd.Context(), globalCfg)
	},
}

func init() {
	rootCmd.AddCommand(workerCmd)

	otelx.MustViperFlags(viper.GetViper(), workerCmd.Flags())
	pubsub.MustViperFlags(viper.GetViper(), workerCmd.Flags())
}

func worker(ctx context.Context, cfg *config.AppConfig) {
	err := otelx.InitTracer(cfg.Tracing, appName, logger)
	if err != nil {
		logger.Fatalw("unable to initialize tracing system", "error", err)
	}

	spiceClient, err := spicedbx.NewClient(cfg.SpiceDB, cfg.Tracing.Enabled)
	if err != nil {
		logger.Fatalw("unable to initialize spicedb client", "error", err)
	}

	engine := query.NewEngine("infratographer", spiceClient)

	logger.Infow("client config", "client_config", cfg.PubSub)

	client := pubsub.NewClient(
		cfg.PubSub,
		pubsub.WithQueryEngine(engine),
		pubsub.WithResourceTypeNames(
			[]string{
				"tenant",
			},
		),
		pubsub.WithLogger(logger),
	)

	if err := client.Listen(); err != nil {
		logger.Fatalw("error listening for events", "error", err)
	}

	// Wait until we're told to stop
	sig := <-sigCh

	logger.Infof("received %s signal, stopping", sig)

	err = client.Stop()
	if err != nil {
		logger.Fatalw("error stopping NATS client", "error", err)
	}
}
