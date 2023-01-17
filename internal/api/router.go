package api

import (
	"strings"

	"github.com/authzed/authzed-go/v1"
	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/otel"
	"go.uber.org/zap"
)

var tracer = otel.Tracer("go.infratographer.com/permissions-api/internal/api")

// Router provides a router for the API
type Router struct {
	// db            *sqlx.DB
	authzedClient *authzed.Client
	logger        *zap.SugaredLogger
}

func NewRouter(authzedClient *authzed.Client, l *zap.SugaredLogger) *Router {
	return &Router{
		authzedClient: authzedClient,
		logger:        l.Named("api"),
	}
}

// Routes will add the routes for this API version to a router group
func (r *Router) Routes(rg *gin.RouterGroup) {
	// /servers
	v1 := rg.Group("api/v1")
	{
		// Creating an OU gets a special
		v1.POST("/resources/:urn", r.resourceCreate)
		v1.PUT("/resources/:urn", r.resourceUpdate)
		v1.DELETE("/resources/:urn", r.resourceDelete)
		// Check resource access
		v1.GET("/available/:type/:scope", r.resourcesAvailable)
		v1.GET("/has/:scope/on/:urn", r.checkScope)
		// Check Global Scope
		v1.GET("/global/check/:scope", r.checkGlobalScope)
	}
}

type actorToken struct {
	// name  string
	// email string
	urn   string
	token string
}

func currentActor(c *gin.Context) (*actorToken, error) {
	authHeader := c.GetHeader("authorization")

	a := &actorToken{}
	a.token = strings.TrimPrefix(authHeader, "bearer ")
	a.urn = a.token

	return a, nil
}
