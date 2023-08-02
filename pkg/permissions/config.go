package permissions

import (
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
	"go.infratographer.com/x/viperx"
)

// Config defines the permissions configuration structure
type Config struct {
	// URL is the URL checks should be executed against
	URL string

	// IgnoreNoResponders will ignore no responder errors when auth relationship requests are published.
	IgnoreNoResponders bool
}

// MustViperFlags adds permissions config flags and viper bindings
func MustViperFlags(v *viper.Viper, flags *pflag.FlagSet) {
	flags.String("permissions-url", "", "sets the permissions url checks should be run against")
	viperx.MustBindFlag(v, "permissions.url", flags.Lookup("permissions-url"))

	flags.String("permissions-ignore-no-responders", "", "ignores no responder errors when auth relationship requests are published")
	viperx.MustBindFlag(v, "permissions.ignoreAuthRelationshipNoResponders", flags.Lookup("permissions-ignore-no-responders"))
}
