// Package config defines the application configuration
package config

import (
	"go.infratographer.com/x/echojwtx"
	"go.infratographer.com/x/echox"
	"go.infratographer.com/x/loggingx"
	"go.infratographer.com/x/otelx"

	"go.infratographer.com/permissions-api/internal/pubsub"
	"go.infratographer.com/permissions-api/internal/spicedbx"
)

// AppConfig is the struct used for configuring the app
type AppConfig struct {
	OIDC    echojwtx.AuthConfig
	Logging loggingx.Config
	Server  echox.Config
	SpiceDB spicedbx.Config
	Tracing otelx.Config
	PubSub  pubsub.Config
}
