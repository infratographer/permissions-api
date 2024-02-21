package api

import (
	"context"
	"net/http"

	"github.com/labstack/echo/v4"
	"go.infratographer.com/x/echojwtx"
	"go.infratographer.com/x/gidx"
	"go.opentelemetry.io/otel"
	"go.uber.org/zap"

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
	v1 := rg.Group("api/v1")
	{
		v1.Use(r.authMW)
		v1.Use(r.injectAPIVersionMW("v1"))

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
		v2.Use(r.injectAPIVersionMW("v2"))

		v2.POST("/resources/:id/roles", r.roleCreate)
		v2.GET("/resources/:id/roles", r.rolesList)
		v2.GET("/roles/:role_id", r.roleGet)
		v2.PATCH("/roles/:role_id", r.roleUpdate)
		v2.DELETE("/roles/:id", r.roleDelete)

		v2.POST("/resources/:id/role-bindings", r.roleBindingCreate)
		v2.GET("/resources/:id/role-bindings", r.roleBindingsList)
		v2.DELETE("/resources/:id/role-bindings", r.roleBindingsDelete)

		v2.GET("/actions", r.listActions)
	}
}

// middleware and utils functions to set and get the API version
const apiVersionContextKey = "apiVersion"

func (r *Router) setAPIVersion(c echo.Context, version string) {
	c.Set(apiVersionContextKey, version)
}

func (r *Router) getAPIVersion(c echo.Context) string {
	return c.Get(apiVersionContextKey).(string)
}

func (r *Router) injectAPIVersionMW(ver string) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			r.setAPIVersion(c, ver)
			return next(c)
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
