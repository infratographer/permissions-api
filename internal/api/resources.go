package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"go.infratographer.com/permissions-api/internal/query"
	"go.infratographer.com/x/urnx"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

func (r *Router) resourceCreate(c *gin.Context) {
	resourceURNStr := c.Param("urn")

	ctx, span := tracer.Start(c.Request.Context(), "api.resourceCreate", trace.WithAttributes(attribute.String("urn", resourceURNStr)))
	defer span.End()

	resourceURN, err := urnx.Parse(resourceURNStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "error processing resource URN", "error": err.Error()})
		return
	}

	resource, err := query.NewResourceFromURN(resourceURN)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "error processing resource URN", "error": err.Error()})
		return
	}

	if err := c.ShouldBindJSON(&resource.Fields); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	actor, err := currentActor(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error(), "message": "failed to get the actor"})
		return
	}

	actorResource, err := query.NewResourceFromURN(actor)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "error processing actor URN", "error": err.Error()})
		return
	}

	zedToken, err := query.CreateSpiceDBRelationships(ctx, r.authzedClient, resource, actorResource)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "failed to create relationship", "error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"token": zedToken})
}

func (r *Router) resourceDelete(c *gin.Context) {
	resourceURN := c.Param("urn")

	_, span := tracer.Start(c.Request.Context(), "api.resourceDelete", trace.WithAttributes(attribute.String("urn", resourceURN)))
	defer span.End()

	c.JSON(http.StatusInternalServerError, gin.H{"message": "not implemented"})
}
