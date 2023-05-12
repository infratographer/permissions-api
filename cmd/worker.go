/*
Copyright © 2022 The Infratographer Authors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

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
