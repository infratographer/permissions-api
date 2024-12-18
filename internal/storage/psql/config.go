package psql

import (
	"net/url"
	"time"

	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

const (
	defaultMaxOpenConns    int           = 25
	defaultMaxIdleConns    int           = 25
	defaultMaxConnLifetime time.Duration = 5 * 60 * time.Second
)

// Config is used to configure a new cockroachdb connection
type Config struct {
	Name        string `mapstructure:"name"`
	Host        string `mapstructure:"host"`
	User        string `mapstructure:"user"`
	Password    string `mapstructure:"password"`
	Params      string `mapstructure:"params"`
	URI         string `mapstructure:"uri"`
	Connections struct {
		MaxOpen     int           `mapstructure:"max_open"`
		MaxIdle     int           `mapstructure:"max_idle"`
		MaxLifetime time.Duration `mapstructure:"max_lifetime"`
	}
}

// GetURI returns the connection URI, if a config URI is provided that will be
// returned, otherwise the host, user, password, and params will be put together
// to make a URI that is returned.
func (c Config) GetURI() string {
	if c.URI != "" {
		return c.URI
	}

	u := url.URL{
		Scheme:   "postgresql",
		User:     url.UserPassword(c.User, c.Password),
		Host:     c.Host,
		Path:     c.Name,
		RawQuery: c.Params,
	}

	return u.String()
}

// MustViperFlags returns the cobra flags and viper config to prevent code duplication
// and help provide consistent flags across the applications
func MustViperFlags(v *viper.Viper, _ *pflag.FlagSet) {
	v.MustBindEnv("psql.host")
	v.MustBindEnv("psql.params")
	v.MustBindEnv("psql.user")
	v.MustBindEnv("psql.password")
	v.MustBindEnv("psql.uri")
	v.MustBindEnv("psql.connections.max_open")
	v.MustBindEnv("psql.connections.max_idle")
	v.MustBindEnv("psql.connections.max_lifetime")

	v.SetDefault("psql.host", "localhost:26257")
	v.SetDefault("psql.connections.max_open", defaultMaxOpenConns)
	v.SetDefault("psql.connections.max_idle", defaultMaxIdleConns)
	v.SetDefault("psql.connections.max_lifetime", defaultMaxConnLifetime)
}

// ConfigFromArgs returns a crdbx.Config from the provided viper-provided
// flags.
func ConfigFromArgs(v *viper.Viper, dbName string) Config {
	cfg := Config{
		Name:     dbName,
		Host:     v.GetString("psql.host"),
		User:     v.GetString("psql.user"),
		Password: v.GetString("psql.password"),
		Params:   v.GetString("psql.params"),
		URI:      v.GetString("psql.uri"),
	}

	cfg.Connections.MaxOpen = v.GetInt("psql.connections.max_open")
	cfg.Connections.MaxIdle = v.GetInt("psql.connections.max_idle")
	cfg.Connections.MaxLifetime = v.GetDuration("psql.connections.max_lifetime")

	return cfg
}
