package pubsub

import (
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
	"go.infratographer.com/x/viperx"
)

const (
	defaultConsumer = "permissions-api-worker"
)

// Config represents a NATS pubsub config.
type Config struct {
	Name        string
	Credentials string
	Server      string
	Stream      string
	Consumer    string
	Prefix      string
}

// MustViperFlags sets required Viper flags for the pubsub package.
func MustViperFlags(v *viper.Viper, flags *pflag.FlagSet) {
	flags.String("pubsub-name", "", "pubsub consumer name")
	viperx.MustBindFlag(viper.GetViper(), "pubsub.name", flags.Lookup("pubsub-name"))

	flags.String("pubsub-credentials", "", "pubsub consumer credentials file")
	viperx.MustBindFlag(viper.GetViper(), "pubsub.credentials", flags.Lookup("pubsub-credentials"))

	flags.String("pubsub-server", "", "pubsub server")
	viperx.MustBindFlag(viper.GetViper(), "pubsub.server", flags.Lookup("pubsub-server"))

	flags.String("pubsub-stream", "", "pubsub stream")
	viperx.MustBindFlag(viper.GetViper(), "pubsub.stream", flags.Lookup("pubsub-stream"))

	flags.String("pubsub-consumer", defaultConsumer, "pubsub consumer")
	viperx.MustBindFlag(viper.GetViper(), "pubsub.consumer", flags.Lookup("pubsub-consumer"))

	flags.String("pubsub-prefix", "", "pubsub subject prefix")
	viperx.MustBindFlag(viper.GetViper(), "pubsub.prefix", flags.Lookup("pubsub-prefix"))
}
