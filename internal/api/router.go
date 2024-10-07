package api

import (
	"context"
	"errors"
	"net/http"

	"github.com/labstack/echo/v4"
	"go.infratographer.com/x/echojwtx"
	"go.infratographer.com/x/gidx"
	"go.opentelemetry.io/otel"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"go.infratographer.com/permissions-api/internal/query"
	"go.infratographer.com/permissions-api/internal/types"
)

var tracer = otel.Tracer("go.infratographer.com/permissions-api/internal/api")

// Router provides a router for the API
type Router struct {
	authMW echo.MiddlewareFunc
	engine query.Engine
	logger *zap.SugaredLogger

	concurrentChecks int
}

// NewRouter returns a new api router
func NewRouter(authCfg echojwtx.AuthConfig, engine query.Engine, options ...Option) (*Router, error) {
	auth, err := echojwtx.NewAuth(context.Background(), authCfg)
	if err != nil {
		return nil, err
	}

	router := &Router{
		authMW: auth.Middleware(),
		engine: engine,
		logger: zap.NewNop().Sugar(),

		concurrentChecks: defaultMaxCheckConcurrency,
	}

	for _, opt := range options {
		if err := opt(router); err != nil {
			return nil, err
		}
	}

	return router, nil
}

// Routes will add the routes for this API version to a router group
func (r *Router) Routes(rg *echo.Group) {
	rg.Use(errorMiddleware)

	v1 := rg.Group("api/v1")
	{
		v1.Use(r.authMW)

		v1.POST("/resources/:id/roles", r.roleCreate)
		v1.GET("/resources/:id/roles", r.rolesList)
		v1.GET("/resources/:id/relationships", r.relationshipListFrom)
		v1.GET("/relationships/from/:id", r.relationshipListFrom)
		v1.GET("/relationships/to/:id", r.relationshipListTo)
		v1.GET("/roles/:role_id", r.roleGet)
		v1.PATCH("/roles/:role_id", r.roleUpdate)
		v1.DELETE("/roles/:id", r.roleDelete)
		v1.GET("/roles/:role_id/resource", r.roleGetResource)
		v1.POST("/roles/:role_id/assignments", r.assignmentCreate)
		v1.DELETE("/roles/:role_id/assignments", r.assignmentDelete)
		v1.GET("/roles/:role_id/assignments", r.assignmentsList)

		// /allow is the permissions check endpoint
		v1.GET("/allow", r.checkAction)
		v1.POST("/allow", r.checkAllActions)
	}

	v2 := rg.Group("api/v2")
	{
		v2.Use(r.authMW)

		v2.POST("/resources/:id/roles", r.roleV2Create)
		v2.GET("/resources/:id/roles", r.roleV2sList)
		v2.GET("/roles/:role_id", r.roleV2Get)
		v2.PATCH("/roles/:role_id", r.roleV2Update)
		v2.DELETE("/roles/:id", r.roleV2Delete)

		v2.GET("/resources/:id/role-bindings", r.roleBindingsList)
		v2.POST("/resources/:id/role-bindings", r.roleBindingCreate)
		v2.GET("/role-bindings/:rb_id", r.roleBindingGet)
		v2.DELETE("/role-bindings/:rb_id", r.roleBindingDelete)
		v2.PATCH("/role-bindings/:rb_id", r.roleBindingUpdate)

		v2.GET("/actions", r.listActions)
	}
}

func errorMiddleware(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		origErr := next(c)

		if origErr == nil {
			return nil
		}

		var (
			checkErr = origErr
			echoMsg  []any
		)

		// If error is an echo.HTTPError, extract it's message to be reused if status is rewritten.
		// Additionally we unwrap the internal error which is then checked instead of the echo error.
		if eerr, ok := origErr.(*echo.HTTPError); ok {
			echoMsg = []any{eerr.Message}
			checkErr = eerr.Internal
		}

		// GRPC returns it's own canceled context status. Here we convert it so we may use the same logic.
		if grpcStatus, ok := status.FromError(checkErr); ok && grpcStatus.Code() == codes.Canceled {
			checkErr = context.Canceled
		}

		switch {
		// Only if the error is a context canceled error and the request context has been canceled.
		// If the request was not canceled, then the context canceled error probably came from the service.
		case errors.Is(checkErr, context.Canceled) && errors.Is(c.Request().Context().Err(), context.Canceled):
			return echo.NewHTTPError(http.StatusUnprocessableEntity, echoMsg...).WithInternal(checkErr)
		default:
			return origErr
		}
	}
}

// Option defines a router option function.
type Option func(r *Router) error

// WithLogger sets the logger for the router.
func WithLogger(logger *zap.SugaredLogger) Option {
	return func(r *Router) error {
		r.logger = logger.Named("api")

		return nil
	}
}

// WithCheckConcurrency sets the check concurrency for bulk permission checks.
func WithCheckConcurrency(count int) Option {
	return func(r *Router) error {
		if count <= 0 {
			count = 5
		}

		r.concurrentChecks = count

		return nil
	}
}

func (r *Router) currentSubject(c echo.Context) (types.Resource, error) {
	subjectStr := echojwtx.Actor(c)

	subject, err := gidx.Parse(subjectStr)
	if err != nil {
		return types.Resource{}, echo.NewHTTPError(http.StatusBadRequest, "failed to get the subject").SetInternal(err)
	}

	subjectResource, err := r.engine.NewResourceFromID(subject)
	if err != nil {
		return types.Resource{}, echo.NewHTTPError(http.StatusBadRequest, "error processing subject ID").SetInternal(err)
	}

	return subjectResource, nil
}
