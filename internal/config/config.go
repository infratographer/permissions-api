package config

import (
	"go.hollow.sh/toolbox/ginjwt"
	"go.infratographer.com/x/ginx"
	"go.infratographer.com/x/loggingx"
	"go.infratographer.com/x/otelx"

	"go.infratographer.com/permissions-api/internal/spicedbx"
)

type AppConfig struct {
	OIDC    ginjwt.AuthConfig
	Logging loggingx.Config
	Server  ginx.Config
	SpiceDB spicedbx.Config
	Tracing otelx.Config
}
