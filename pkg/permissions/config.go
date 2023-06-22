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
}

// MustViperFlags adds permissions config flags and viper bindings
func MustViperFlags(v *viper.Viper, flags *pflag.FlagSet) {
	flags.String("permissions-url", "", "sets the permissions url checks should be run against")
	viperx.MustBindFlag(v, "permissions.url", flags.Lookup("permissions-url"))
}
