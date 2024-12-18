package cmd

import (
	"context"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"go.infratographer.com/x/gidx"
	"go.infratographer.com/x/viperx"

	"go.infratographer.com/permissions-api/internal/config"
	"go.infratographer.com/permissions-api/internal/iapl"
	"go.infratographer.com/permissions-api/internal/query"
	"go.infratographer.com/permissions-api/internal/spicedbx"
	"go.infratographer.com/permissions-api/internal/storage"
	"go.infratographer.com/permissions-api/internal/types"
)

const (
	createRoleFlagSubject  = "subject"
	createRoleFlagResource = "resource"
	createRoleFlagActions  = "actions"
	createRoleFlagName     = "name"
)

var createRoleCmd = &cobra.Command{
	Use:   "create-role",
	Short: "create role in SpiceDB directly",
	Run: func(cmd *cobra.Command, _ []string) {
		createRole(cmd.Context(), globalCfg)
	},
}

func init() {
	rootCmd.AddCommand(createRoleCmd)

	flags := createRoleCmd.Flags()
	flags.String(createRoleFlagSubject, "", "subject to assign to created role")
	flags.StringSlice(createRoleFlagActions, []string{}, "actions to assign to created role")
	flags.String(createRoleFlagResource, "", "resource to bind to created role")
	flags.String(createRoleFlagName, "", "name of role to create")

	v := viper.GetViper()

	viperx.MustBindFlag(v, createRoleFlagSubject, flags.Lookup(createRoleFlagSubject))
	viperx.MustBindFlag(v, createRoleFlagActions, flags.Lookup(createRoleFlagActions))
	viperx.MustBindFlag(v, createRoleFlagResource, flags.Lookup(createRoleFlagResource))
	viperx.MustBindFlag(v, createRoleFlagName, flags.Lookup(createRoleFlagName))
}

func createRole(ctx context.Context, cfg *config.AppConfig) {
	subjectIDStr := viper.GetString(createRoleFlagSubject)
	actions := viper.GetStringSlice(createRoleFlagActions)
	resourceIDStr := viper.GetString(createRoleFlagResource)
	name := viper.GetString(createRoleFlagName)

	if subjectIDStr == "" || len(actions) == 0 || resourceIDStr == "" || name == "" {
		logger.Fatal("invalid config")
	}

	spiceClient, err := spicedbx.NewClient(cfg.SpiceDB, cfg.Tracing.Enabled)
	if err != nil {
		logger.Fatalw("unable to initialize spicedb client", "error", err)
	}

	db, err := newDBFromConfig(cfg)
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

	resourceID, err := gidx.Parse(resourceIDStr)
	if err != nil {
		logger.Fatalw("error parsing resource ID", "error", err)
	}

	subjectID, err := gidx.Parse(subjectIDStr)
	if err != nil {
		logger.Fatalw("error parsing subject ID", "error", err)
	}

	engine, err := query.NewEngine("infratographer", spiceClient, store, query.WithPolicy(policy), query.WithLogger(logger))
	if err != nil {
		logger.Fatalw("error creating engine", "error", err)
	}

	resource, err := engine.NewResourceFromID(resourceID)
	if err != nil {
		logger.Fatalw("error creating resource", "error", err)
	}

	subjectResource, err := engine.NewResourceFromID(subjectID)
	if err != nil {
		logger.Fatalw("error creating subject resource", "error", err)
	}

	role, err := engine.CreateRoleV2(ctx, subjectResource, resource, "", name, actions)
	if err != nil {
		logger.Fatalw("error creating role", "error", err)
	}

	rbsubj := []types.RoleBindingSubject{{SubjectResource: subjectResource}}

	roleres, err := engine.NewResourceFromID(role.ID)
	if err != nil {
		logger.Fatalw("error creating role resource", "error", err)
	}

	rb, err := engine.CreateRoleBinding(ctx, subjectResource, resource, roleres, "", rbsubj)
	if err != nil {
		logger.Fatalw("error creating role binding", "error", err)
	}

	logger.Infof("created role %s[%s] and role-binding %s", role.Name, role.ID, rb.ID)
}
