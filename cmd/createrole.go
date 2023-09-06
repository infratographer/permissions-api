package cmd

import (
	"context"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"go.infratographer.com/permissions-api/internal/config"
	"go.infratographer.com/permissions-api/internal/iapl"
	"go.infratographer.com/permissions-api/internal/query"
	"go.infratographer.com/permissions-api/internal/spicedbx"
	"go.infratographer.com/x/gidx"
	"go.infratographer.com/x/viperx"
)

const (
	createRoleFlagSubject  = "subject"
	createRoleFlagResource = "resource"
	createRoleFlagActions  = "actions"
)

var (
	createRoleCmd = &cobra.Command{
		Use:   "create-role",
		Short: "create role in SpiceDB directly",
		Run: func(cmd *cobra.Command, args []string) {
			createRole(cmd.Context(), globalCfg)
		},
	}
)

func init() {
	rootCmd.AddCommand(createRoleCmd)

	flags := createRoleCmd.Flags()
	flags.String(createRoleFlagSubject, "", "subject to assign to created role")
	flags.StringSlice(createRoleFlagActions, []string{}, "actions to assign to created role")
	flags.String(createRoleFlagResource, "", "resource to bind to created role")

	v := viper.GetViper()

	viperx.MustBindFlag(v, createRoleFlagSubject, flags.Lookup(createRoleFlagSubject))
	viperx.MustBindFlag(v, createRoleFlagActions, flags.Lookup(createRoleFlagActions))
	viperx.MustBindFlag(v, createRoleFlagResource, flags.Lookup(createRoleFlagResource))
}

func createRole(ctx context.Context, cfg *config.AppConfig) {
	subjectIDStr := viper.GetString(createRoleFlagSubject)
	actions := viper.GetStringSlice(createRoleFlagActions)
	resourceIDStr := viper.GetString(createRoleFlagResource)

	if subjectIDStr == "" || len(actions) == 0 || resourceIDStr == "" {
		logger.Fatal("invalid config")
	}

	spiceClient, err := spicedbx.NewClient(cfg.SpiceDB, cfg.Tracing.Enabled)
	if err != nil {
		logger.Fatalw("unable to initialize spicedb client", "error", err)
	}

	var policy iapl.Policy

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

	resourceID, err := gidx.Parse(resourceIDStr)
	if err != nil {
		logger.Fatalw("error parsing resource ID", "error", err)
	}

	subjectID, err := gidx.Parse(subjectIDStr)
	if err != nil {
		logger.Fatalw("error parsing subject ID", "error", err)
	}

	engine := query.NewEngine("infratographer", spiceClient, query.WithPolicy(policy), query.WithLogger(logger))

	resource, err := engine.NewResourceFromID(resourceID)
	if err != nil {
		logger.Fatalw("error creating resource", "error", err)
	}

	subjectResource, err := engine.NewResourceFromID(subjectID)
	if err != nil {
		logger.Fatalw("error creating subject resource", "error", err)
	}

	role, _, err := engine.CreateRole(ctx, resource, actions)
	if err != nil {
		logger.Fatalw("error creating role", "error", err)
	}

	_, err = engine.AssignSubjectRole(ctx, subjectResource, role)
	if err != nil {
		logger.Fatalw("error creating role", "error", err)
	}

	logger.Infow("role successfully created", "role_id", role.ID)
}
