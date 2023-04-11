package api

import (
	"github.com/gin-gonic/gin"
	"go.hollow.sh/toolbox/ginjwt"
	"go.infratographer.com/permissions-api/internal/query"
	"go.infratographer.com/x/urnx"
	"go.opentelemetry.io/otel"
	"go.uber.org/zap"
)

var tracer = otel.Tracer("go.infratographer.com/permissions-api/internal/api")

// Router provides a router for the API
type Router struct {
	authMW func(*gin.Context)
	engine *query.Engine
	logger *zap.SugaredLogger
}

// NewRouter returns a new api router
func NewRouter(authCfg ginjwt.AuthConfig, engine *query.Engine, l *zap.SugaredLogger) (*Router, error) {
	authMW, err := newAuthMiddleware(authCfg)
	if err != nil {
		return nil, err
	}

	out := &Router{
		authMW: authMW,
		engine: engine,
		logger: l.Named("api"),
	}

	return out, nil
}

// Routes will add the routes for this API version to a router group
func (r *Router) Routes(rg *gin.RouterGroup) {
	// /servers
	v1 := rg.Group("api/v1").Use(r.authMW)
	{
		v1.POST("/resources/:urn/roles", r.roleCreate)
		v1.GET("/resources/:urn/roles", r.rolesList)
		v1.POST("/resources/:urn/relationships", r.relationshipsCreate)
		v1.GET("/resources/:urn/relationships", r.relationshipsList)
		v1.POST("/roles/:role_id/assignments", r.assignmentCreate)
		v1.GET("/roles/:role_id/assignments", r.assignmentsList)

		// /allow is the permissions check endpoint
		v1.GET("/allow", r.checkAction)
	}
}

func currentSubject(c *gin.Context) (*urnx.URN, error) {
	subject := ginjwt.GetSubject(c)

	return urnx.Parse(subject)
}
