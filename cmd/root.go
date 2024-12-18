package cmd

import (
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"go.infratographer.com/x/crdbx"
	"go.infratographer.com/x/goosex"
	"go.infratographer.com/x/loggingx"
	"go.infratographer.com/x/versionx"
	"go.infratographer.com/x/viperx"
	"go.uber.org/zap"

	"go.infratographer.com/permissions-api/internal/config"
	"go.infratographer.com/permissions-api/internal/storage"
	"go.infratographer.com/permissions-api/internal/storage/psql"
)

var (
	appName   = "permissions-api"
	cfgFile   string
	logger    *zap.SugaredLogger
	globalCfg *config.AppConfig
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   appName,
	Short: "Infratographer Permissions API Service",
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func init() {
	cobra.OnInitialize(initConfig)
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is /etc/infratographer/permissions-api.yaml)")
	loggingx.MustViperFlags(viper.GetViper(), rootCmd.PersistentFlags())

	// Database Flags
	crdbx.MustViperFlags(viper.GetViper(), rootCmd.Flags())
	psql.MustViperFlags(viper.GetViper(), rootCmd.Flags())

	// Add migrate command
	goosex.RegisterCobraCommand(rootCmd, func() {
		goosex.SetLogger(logger)

		logger.Infow("setting up migrations", "engine", string(globalCfg.DB.Engine))

		switch globalCfg.DB.Engine {
		case config.DBEnginePostgreSQL:
			goosex.SetBaseFS(psql.Migrations)
			goosex.SetDBURI(globalCfg.PSQL.GetURI())
		case config.DBEngineCockroachDB:
			goosex.SetBaseFS(psql.Migrations)
			goosex.SetDBURI(globalCfg.CRDB.GetURI())
		default:
			log.Fatalf("unknown database engine: %s", globalCfg.DB.Engine)
		}
	})

	// Add version command
	versionx.RegisterCobraCommand(rootCmd, func() { versionx.PrintVersion(logger) })

	rootCmd.PersistentFlags().String("spicedb-endpoint", "spicedb:50051", "spicedb endpoint (host:port)")
	viperx.MustBindFlag(viper.GetViper(), "spicedb.endpoint", rootCmd.PersistentFlags().Lookup("spicedb-endpoint"))
	rootCmd.PersistentFlags().String("spicedb-key", "", "spicedb auth key")
	viperx.MustBindFlag(viper.GetViper(), "spicedb.key", rootCmd.PersistentFlags().Lookup("spicedb-key"))
	rootCmd.PersistentFlags().Bool("spicedb-insecure", false, "spicedb insecure connection")
	viperx.MustBindFlag(viper.GetViper(), "spicedb.insecure", rootCmd.PersistentFlags().Lookup("spicedb-insecure"))
	rootCmd.PersistentFlags().Bool("spicedb-verifyca", false, "spicedb verify CA cert for secure connections")
	viperx.MustBindFlag(viper.GetViper(), "spicedb.verifyca", rootCmd.PersistentFlags().Lookup("spicedb-verifyca"))
	rootCmd.PersistentFlags().String("spicedb-prefix", "", "spicedb prefix")
	viperx.MustBindFlag(viper.GetViper(), "spicedb.prefix", rootCmd.PersistentFlags().Lookup("spicedb-prefix"))
	rootCmd.PersistentFlags().String("spicedb-policydir", "", "spicedb policy directory")
	viperx.MustBindFlag(viper.GetViper(), "spicedb.policyDir", rootCmd.PersistentFlags().Lookup("spicedb-policydir"))

	rootCmd.PersistentFlags().String("db-engine", "cockoach", "database engine to use (cockroach, postgres)")
	viperx.MustBindFlag(viper.GetViper(), "db.engine", rootCmd.PersistentFlags().Lookup("db-engine"))
}

// initConfig reads in config file and ENV variables if set.
func initConfig() {
	if cfgFile != "" {
		// Use config file from the flag.
		viper.SetConfigFile(cfgFile)
	} else {
		viper.AddConfigPath("/etc/infratographer/")
		viper.SetConfigType("yaml")
		viper.SetConfigName(appName)
	}

	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	viper.SetEnvPrefix("permissionsapi")
	viper.AutomaticEnv() // read in environment variables that match

	// If a config file is found, read it in.
	err := viper.ReadInConfig()

	var settings config.AppConfig

	if err := viper.Unmarshal(&settings); err != nil {
		log.Fatalf("unable to process app config, error: %s", err.Error())
	}

	logger = loggingx.InitLogger(appName, settings.Logging)

	// errcheck for ReadInConfig, but we have to initialize the logger and
	if err == nil {
		logger.Infow("using config file",
			"file", viper.ConfigFileUsed(),
		)
	}

	globalCfg = &settings
}

// newDBFromConfig creates a new database connection based on the provided configuration.
func newDBFromConfig(cfg *config.AppConfig) (storage.DB, error) {
	logger.Infow("setting up database", "engine", string(cfg.DB.Engine))

	switch cfg.DB.Engine {
	case config.DBEnginePostgreSQL:
		return psql.NewDB(cfg.PSQL, cfg.Tracing.Enabled)
	case config.DBEngineCockroachDB:
		return crdbx.NewDB(cfg.CRDB, cfg.Tracing.Enabled)
	default:
		return nil, fmt.Errorf("%w: %s", ErrUnsupportedDBEngine, cfg.DB.Engine)
	}
}
