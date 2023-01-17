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
	"log"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"go.infratographer.com/x/loggingx"
	"go.infratographer.com/x/versionx"
	"go.infratographer.com/x/viperx"
	"go.uber.org/zap"

	"go.infratographer.com/permissions-api/internal/config"
)

var (
	appName = "permissionapi"
	cfgFile string
	logger  *zap.SugaredLogger
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "infra-permission-api",
	Short: "Infratographer Permission API Service",
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
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is /etc/infratographer/permission-api.yaml)")
	loggingx.MustViperFlags(rootCmd.PersistentFlags())

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
	viper.SetEnvPrefix(appName)
	viper.AutomaticEnv() // read in environment variables that match

	// If a config file is found, read it in.
	err := viper.ReadInConfig()

	var settings config.Settings

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

	config.AppConfig = settings
}
