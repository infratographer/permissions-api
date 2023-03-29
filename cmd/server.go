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
