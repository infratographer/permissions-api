package permissions

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/pkg/errors"
	"go.infratographer.com/x/echojwtx"
	"go.infratographer.com/x/events"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.uber.org/zap"
)

const (
	bearerPrefix = "Bearer "

	defaultClientTimeout = 5 * time.Second

	outcomeAllowed = "allowed"
	outcomeDenied  = "denied"
)

var (
	defaultClient = &http.Client{
		Timeout:   defaultClientTimeout,
		Transport: otelhttp.NewTransport(http.DefaultTransport),
	}

	tracer = otel.GetTracerProvider().Tracer("go.infratographer.com/permissions-api/pkg/permissions")
)

// Permissions handles supporting authorization checks
type Permissions struct {
	enableChecker bool

	logger             *zap.SugaredLogger
	publisher          events.AuthRelationshipPublisher
	client             *http.Client
	url                *url.URL
	skipper            middleware.Skipper
	defaultChecker     Checker
	ignoreNoResponders bool
}

// Middleware produces echo middleware to handle authorization checks
func (p *Permissions) Middleware() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			setAuthRelationshipRequestHandler(c, p)

			if !p.enableChecker || p.skipper(c) {
				setCheckerContext(c, p.defaultChecker)

				return next(c)
			}

			actor := echojwtx.Actor(c)
			if actor == "" {
				return ErrNoAuthToken
			}

			authHeader := strings.TrimSpace(c.Request().Header.Get(echo.HeaderAuthorization))

			if len(authHeader) <= len(bearerPrefix) {
				return ErrInvalidAuthToken
			}

			if !strings.EqualFold(authHeader[:len(bearerPrefix)], bearerPrefix) {
				return ErrInvalidAuthToken
			}

			token := authHeader[len(bearerPrefix):]

			setCheckerContext(c, p.checker(c, actor, token))

			return next(c)
		}
	}
}

type checkPermissionRequest struct {
	Actions []AccessRequest `json:"actions"`
}

func (p *Permissions) checker(c echo.Context, actor, token string) Checker {
	return func(ctx context.Context, requests ...AccessRequest) error {
		ctx, span := tracer.Start(ctx, "permissions.checker")
		defer span.End()

		span.SetAttributes(
			attribute.String("permissions.actor", actor),
			attribute.Int("permissions.requests", len(requests)),
		)

		logger := p.logger.With("actor", actor, "requests", len(requests))

		request := checkPermissionRequest{
			Actions: requests,
		}

		var reqBody bytes.Buffer

		if err := json.NewEncoder(&reqBody).Encode(request); err != nil {
			err = errors.WithStack(err)

			span.SetStatus(codes.Error, err.Error())
			logger.Errorw("failed to encode request body", "error", err)

			return err
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.url.String(), &reqBody)
		if err != nil {
			err = errors.WithStack(err)

			span.SetStatus(codes.Error, err.Error())
			logger.Errorw("failed to create checker request", "error", err)

			return err
		}

		req.Header.Set(echo.HeaderAuthorization, c.Request().Header.Get(echo.HeaderAuthorization))
		req.Header.Set(echo.HeaderContentType, "application/json")

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
				span.SetAttributes(
					attribute.String(
						"permissions.outcome",
						outcomeDenied,
					),
				)
			case errors.Is(err, ErrBadResponse):
				logger.Errorw("bad response from server", "error", err, "response.status_code", resp.StatusCode, "response.body", string(body))
				span.SetStatus(codes.Error, errors.WithStack(err).Error())
			}

			return err
		}

		span.SetAttributes(
			attribute.String(
				"permissions.outcome",
				outcomeAllowed,
			),
		)
		logger.Debug("access granted to resource")

		return nil
	}
}

// New creates a new Permissions instance
func New(config Config, options ...Option) (*Permissions, error) {
	p := &Permissions{
		enableChecker:      config.URL != "",
		client:             defaultClient,
		skipper:            middleware.DefaultSkipper,
		defaultChecker:     DefaultDenyChecker,
		ignoreNoResponders: config.IgnoreNoResponders,
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

func ensureValidServerResponse(resp *http.Response) error {
	if resp.StatusCode >= http.StatusMultiStatus {
		if resp.StatusCode == http.StatusForbidden {
			return ErrPermissionDenied
		}

		return fmt.Errorf("%w: %d", ErrBadResponse, resp.StatusCode)
	}

	return nil
}
