package permissions

import (
	"net/http"
	"time"

	"github.com/hashicorp/go-retryablehttp"
	"github.com/labstack/echo/v4/middleware"
	"go.infratographer.com/x/events"
	"go.uber.org/zap"

	"go.infratographer.com/permissions-api/pkg/permissions/internal/selecthost"
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
		c := retryablehttp.NewClient()

		c.RetryWaitMin = 100 * time.Millisecond //nolint: mnd
		c.RetryWaitMax = 2 * time.Second        //nolint: mnd

		discoveryOpts := append([]selecthost.Option{
			selecthost.Logger(p.logger),
		}, p.discoveryOpts...)

		transport, err := p.config.initTransport(client.Transport, discoveryOpts...)
		if err != nil {
			return err
		}

		c.HTTPClient = &http.Client{
			Transport:     transport,
			CheckRedirect: client.CheckRedirect,
			Jar:           client.Jar,
			Timeout:       client.Timeout,
		}

		p.client = c

		return nil
	}
}

// WithDiscoveryOptions provides additional select host discovery options
func WithDiscoveryOptions(opts ...selecthost.Option) Option {
	return func(p *Permissions) error {
		p.discoveryOpts = append(p.discoveryOpts, opts...)

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
