// Package config defines the application configuration
package config

import (
	"go.hollow.sh/toolbox/ginjwt"
	"go.infratographer.com/x/ginx"
	"go.infratographer.com/x/loggingx"
	"go.infratographer.com/x/otelx"

	"go.infratographer.com/permissions-api/internal/spicedbx"
)

// AppConfig is the struct used for configuring the app
type AppConfig struct {
	OIDC    ginjwt.AuthConfig
	Logging loggingx.Config
	Server  ginx.Config
	SpiceDB spicedbx.Config
	Tracing otelx.Config
}
