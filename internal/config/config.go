package config

import (
	"go.infratographer.com/x/ginx"
	"go.infratographer.com/x/loggingx"
	"go.infratographer.com/x/otelx"

	"go.infratographer.com/permissions-api/internal/spicedbx"
)

var AppConfig struct {
	Logging loggingx.Config
	Server  ginx.Config
	SpiceDB spicedbx.Config
	Tracing otelx.Config
}
