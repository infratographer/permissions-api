package permissions

import (
	"net/http"

	"github.com/labstack/echo/v4/middleware"
	"go.infratographer.com/x/events"
	"go.uber.org/zap"
)

// Option defines an option configurator
type Option func(p *Permissions) error

// WithLogger sets the logger for the auth handler
func WithLogger(logger *zap.SugaredLogger) Option {
	return func(p *Permissions) error {
		p.logger = logger

		return nil
	}
}

// WithEventsPublisher sets the underlying event publisher the auth handler uses
func WithEventsPublisher(publisher events.AuthRelationshipPublisher) Option {
	return func(p *Permissions) error {
		p.publisher = publisher

		return nil
	}
}

// WithHTTPClient sets the underlying http client the auth handler uses
func WithHTTPClient(client *http.Client) Option {
	return func(p *Permissions) error {
		p.client = client

		return nil
	}
}

// WithSkipper sets the echo middleware skipper function
func WithSkipper(skipper middleware.Skipper) Option {
	return func(p *Permissions) error {
		p.skipper = skipper

		return nil
	}
}

// WithDefaultChecker sets the default checker if the middleware is skipped
func WithDefaultChecker(checker Checker) Option {
	return func(p *Permissions) error {
		p.defaultChecker = checker

		return nil
	}
}
