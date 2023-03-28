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
	"fmt"

	v1 "github.com/authzed/authzed-go/proto/authzed/api/v1"
	"github.com/spf13/cobra"
	"go.infratographer.com/x/otelx"

	"go.infratographer.com/permissions-api/internal/config"
	"go.infratographer.com/permissions-api/internal/spicedbx"
)

var (
	schemaCmd = &cobra.Command{
		Use:   "schema",
		Short: "write the schema into SpiceDB",
		Run: func(cmd *cobra.Command, args []string) {
			writeSchema(cmd.Context(), dryRun, globalCfg)
		},
	}

	dryRun bool
)

func init() {
	rootCmd.AddCommand(schemaCmd)

	schemaCmd.Flags().BoolVar(&dryRun, "dry-run", false, "dry run: print the schema instead of applying it")
}

func writeSchema(ctx context.Context, dryRun bool, cfg *config.AppConfig) {
	schemaStr := spicedbx.GeneratedSchema("infratographer")

	if dryRun {
		fmt.Printf("%s", schemaStr)
		return
	}

	err := otelx.InitTracer(cfg.Tracing, appName, logger)
	if err != nil {
		logger.Fatalw("unable to initialize tracing system", "error", err)
	}

	client, err := spicedbx.NewClient(cfg.SpiceDB, cfg.Tracing.Enabled)
	if err != nil {
		logger.Fatalw("unable to initialize spicedb client", "error", err)
	}

	logger.Debugw("Writing schema to DB", "schema", schemaStr)

	_, err = client.WriteSchema(context.Background(), &v1.WriteSchemaRequest{Schema: schemaStr})
	if err != nil {
		logger.Fatalw("error writing schema to SpiceDB", "error", err)
	}

	logger.Info("schema applied to SpiceDB")
}
