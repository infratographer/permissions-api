package permissions

import (
	"context"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/pkg/errors"
	"go.infratographer.com/x/echojwtx"
	"go.infratographer.com/x/gidx"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.uber.org/zap"
)

const (
	bearerPrefix = "Bearer "

	defaultClientTimeout = 5 * time.Second
)

var (
	// CheckerCtxKey is the context key used to set the checker handling function
	CheckerCtxKey = checkerCtxKey{}

	// DefaultAllowChecker defaults to allow when checker is disabled or skipped
	DefaultAllowChecker Checker = func(_ context.Context, _ gidx.PrefixedID, _ string) error {
		return nil
	}

	// DefaultDenyChecker defaults to denied when checker is disabled or skipped
	DefaultDenyChecker Checker = func(_ context.Context, _ gidx.PrefixedID, _ string) error {
		return ErrPermissionDenied
	}

	defaultClient = &http.Client{
		Timeout:   defaultClientTimeout,
		Transport: otelhttp.NewTransport(http.DefaultTransport),
	}

	tracer = otel.GetTracerProvider().Tracer("go.infratographer.com/permissions-api/pkg/permissions")
)

// Checker defines the checker function definition
type Checker func(ctx context.Context, resource gidx.PrefixedID, action string) error

type checkerCtxKey struct{}

// Permissions handles supporting authorization checks
type Permissions struct {
	enabled        bool
	logger         *zap.SugaredLogger
	client         *http.Client
	url            *url.URL
	skipper        middleware.Skipper
	defaultChecker Checker
}

// Middleware produces echo middleware to handle authorization checks
func (p *Permissions) Middleware() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			if !p.enabled || p.skipper(c) {
				setCheckerContext(c, p.defaultChecker)

				return next(c)
			}

			actor := echojwtx.Actor(c)
			if actor == "" {
				return echo.ErrUnauthorized.WithInternal(ErrNoAuthToken)
			}

			authHeader := strings.TrimSpace(c.Request().Header.Get(echo.HeaderAuthorization))

			if len(authHeader) <= len(bearerPrefix) {
				return echo.ErrUnauthorized.WithInternal(ErrInvalidAuthToken)
			}

			if !strings.EqualFold(authHeader[:len(bearerPrefix)], bearerPrefix) {
				return echo.ErrUnauthorized.WithInternal(ErrInvalidAuthToken)
			}

			token := authHeader[len(bearerPrefix):]

			setCheckerContext(c, p.checker(c, actor, token))

			return next(c)
		}
	}
}

func (p *Permissions) checker(c echo.Context, actor, token string) Checker {
	return func(ctx context.Context, resource gidx.PrefixedID, action string) error {
		ctx, span := tracer.Start(ctx, "permissions.checkAccess")
		defer span.End()

		span.SetAttributes(
			attribute.String("permissions.actor", actor),
			attribute.String("permissions.action", action),
			attribute.String("permissions.resource", resource.String()),
		)

		logger := p.logger.With("actor", actor, "resource", resource.String(), "action", action)

		values := url.Values{}
		values.Add("resource", resource.String())
		values.Add("action", action)

		url := *p.url
		url.RawQuery = values.Encode()

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url.String(), nil)
		if err != nil {
			span.SetStatus(codes.Error, errors.WithStack(err).Error())
			logger.Errorw("failed to create checker request", "error", err)

			return errors.WithStack(err)
		}

		req.Header.Set(echo.HeaderAuthorization, c.Request().Header.Get(echo.HeaderAuthorization))

		resp, err := p.client.Do(req)
		if err != nil {
			err = errors.WithStack(err)

			logger.Errorw("failed to make request", "error", err)

			return err
		}

		defer resp.Body.Close()

		err = ensureValidServerResponse(resp)
		if err != nil {
			body, _ := io.ReadAll(resp.Body) //nolint:errcheck // ignore any errors reading as this is just for logging.

			switch {
			case errors.Is(err, ErrPermissionDenied):
				logger.Warnw("unauthorized access to resource")
				span.AddEvent("permission denied")
			case errors.Is(err, ErrBadResponse):
				logger.Errorw("bad response from server", "error", err, "response.status_code", resp.StatusCode, "response.body", string(body))
				span.SetStatus(codes.Error, errors.WithStack(err).Error())
			}

			return err
		}

		logger.Debug("access granted to resource")

		return nil
	}
}

// New creates a new Permissions instance
func New(config Config, options ...Option) (*Permissions, error) {
	p := &Permissions{
		enabled:        config.URL != "",
		client:         defaultClient,
		skipper:        middleware.DefaultSkipper,
		defaultChecker: DefaultDenyChecker,
	}

	if config.URL != "" {
		uri, err := url.Parse(config.URL)
		if err != nil {
			return nil, err
		}

		p.url = uri
	}

	for _, opt := range options {
		if err := opt(p); err != nil {
			return nil, err
		}
	}

	if p.logger == nil {
		p.logger = zap.NewNop().Sugar()
	}

	return p, nil
}

func setCheckerContext(c echo.Context, checker Checker) {
	if checker == nil {
		checker = DefaultDenyChecker
	}

	req := c.Request().WithContext(
		context.WithValue(
			c.Request().Context(),
			CheckerCtxKey,
			checker,
		),
	)

	c.SetRequest(req)
}

func ensureValidServerResponse(resp *http.Response) error {
	if resp.StatusCode >= http.StatusMultiStatus {
		if resp.StatusCode == http.StatusForbidden {
			return ErrPermissionDenied
		}

		return ErrBadResponse
	}

	return nil
}

// CheckAccess runs the checker function to check if the provided resource and action are supported.
func CheckAccess(ctx context.Context, resource gidx.PrefixedID, action string) error {
	checker, ok := ctx.Value(CheckerCtxKey).(Checker)
	if !ok {
		return ErrCheckerNotFound
	}

	return checker(ctx, resource, action)
}
