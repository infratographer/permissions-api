package api

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"go.infratographer.com/permissions-api/internal/query"
	"go.infratographer.com/x/urnx"
)

func (r *Router) checkScope(c *gin.Context) {
	resourceURNStr := c.Param("urn")
	scope := c.Param("scope")

	ctx, span := tracer.Start(c.Request.Context(), "api.checkScope")
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

	err = query.ActorHasPermission(ctx, r.authzedClient, actorResource, scope, resource, "")
	if err != nil {
		if errors.Is(err, query.ErrScopeNotAssigned) {
			c.JSON(http.StatusForbidden, gin.H{"message": "actor does not have requested scope"})
			return
		}

		c.JSON(http.StatusInternalServerError, gin.H{"message": "an error occurred checking permissions", "error": err.Error()})

		return
	}

	c.JSON(http.StatusOK, gin.H{})
}
