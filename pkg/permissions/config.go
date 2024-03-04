package permissions

import (
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
	"go.infratographer.com/x/viperx"
)

// Config defines the permissions configuration structure
type Config struct {
	// URL should point to a permissions-api authorization API route, such as https://example.com/api/v1/allow.
	// If not set, all permissions checks will be denied by default. To override this behavior, set DefaultAllow
	// to true.
	URL string

	// IgnoreNoResponders will ignore no responder errors when auth relationship requests are published.
	IgnoreNoResponders bool

	// DefaultAllow if set to true, will allow all permissions checks when URL is not set.
	DefaultAllow bool
}

// MustViperFlags adds permissions config flags and viper bindings
func MustViperFlags(v *viper.Viper, flags *pflag.FlagSet) {
	flags.String("permissions-url", "", "sets the permissions url checks should be run against")
	viperx.MustBindFlag(v, "permissions.url", flags.Lookup("permissions-url"))

	flags.Bool("permissions-ignore-no-responders", false, "ignores no responder errors when auth relationship requests are published")
	viperx.MustBindFlag(v, "permissions.ignoreNoResponders", flags.Lookup("permissions-ignore-no-responders"))

	flags.Bool("permissions-default-allow", false, "grant permission checks when url is not set")
	viperx.MustBindFlag(v, "permissions.defaultAllow", flags.Lookup("permissions-default-allow"))
}
