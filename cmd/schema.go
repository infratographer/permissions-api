package cmd

import (
	"context"
	"fmt"

	v1 "github.com/authzed/authzed-go/proto/authzed/api/v1"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"go.infratographer.com/x/otelx"

	"go.infratographer.com/permissions-api/internal/config"
	"go.infratographer.com/permissions-api/internal/iapl"
	"go.infratographer.com/permissions-api/internal/spicedbx"
)

var (
	schemaCmd = &cobra.Command{
		Use:   "schema",
		Short: "write the schema into SpiceDB",
		Run: func(cmd *cobra.Command, _ []string) {
			writeSchema(cmd.Context(), dryRun, globalCfg)
		},
	}

	dryRun bool
)

func init() {
	rootCmd.AddCommand(schemaCmd)

	schemaCmd.Flags().BoolVar(&dryRun, "dry-run", false, "dry run: print the schema instead of applying it")

	schemaCmd.Flags().Bool("mermaid", false, "outputs the policy as a mermaid chart definition")
	schemaCmd.Flags().Bool("mermaid-markdown", false, "outputs the policy as a markdown mermaid chart definition")

	if err := viper.BindPFlag("mermaid", schemaCmd.Flags().Lookup("mermaid")); err != nil {
		panic(err)
	}

	if err := viper.BindPFlag("mermaid-markdown", schemaCmd.Flags().Lookup("mermaid-markdown")); err != nil {
		panic(err)
	}
}

func writeSchema(_ context.Context, dryRun bool, cfg *config.AppConfig) {
	var (
		err    error
		policy iapl.Policy
	)

	if cfg.SpiceDB.PolicyFile != "" {
		policy, err = iapl.NewPolicyFromFile(cfg.SpiceDB.PolicyFile)
		if err != nil {
			logger.Fatalw("unable to load new policy from schema file", "policy_file", cfg.SpiceDB.PolicyFile, "error", err)
		}
	} else {
		logger.Warn("no spicedb policy file defined, using default policy")

		policy = iapl.DefaultPolicy()
	}

	if err = policy.Validate(); err != nil {
		logger.Fatalw("invalid spicedb policy", "error", err)
	}

	schemaStr, err := spicedbx.GenerateSchema("infratographer", policy.Schema())
	if err != nil {
		logger.Fatalw("failed to generate schema from policy", "error", err)
	}

	if viper.GetBool("mermaid") || viper.GetBool("mermaid-markdown") {
		outputPolicyMermaid(cfg.SpiceDB.PolicyFile, viper.GetBool("mermaid-markdown"))

		return
	}

	if dryRun {
		fmt.Printf("%s", schemaStr)
		return
	}

	err = otelx.InitTracer(cfg.Tracing, appName, logger)
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
