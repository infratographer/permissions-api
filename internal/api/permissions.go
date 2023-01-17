package api

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"go.infratographer.com/permissions-api/internal/query"
)

func (r *Router) checkGlobalScope(c *gin.Context) {
	scope := c.Param("scope")

	ctx, span := tracer.Start(c.Request.Context(), "api.checkGlobalScope")
	defer span.End()

	actor, err := currentActor(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error(), "message": "failed to get the actor"})
		return
	}

	actorResource, err := query.NewResourceFromURN(actor.urn)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "error processing actor URN", "error": err.Error()})
		return
	}

	err = query.ActorHasGlobalPermission(ctx, r.authzedClient, actorResource, scope, "")
	if err != nil {
		if errors.Is(err, query.ErrScopeNotAssigned) {
			c.JSON(http.StatusForbidden, gin.H{"message": "actor does not have requested scope"})
			return
		}

		c.JSON(http.StatusInternalServerError, gin.H{"message": "an error occurred checking actor scopes", "error": err.Error()})

		return
	}

	c.JSON(http.StatusOK, gin.H{})
}

func (r *Router) resourcesAvailable(c *gin.Context) {
	ctx, span := tracer.Start(c.Request.Context(), "api.resourcesAvailable")
	defer span.End()

	resourceURN := c.Param("type")
	scope := c.Param("scope")

	actor, err := currentActor(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error(), "message": "failed to get the actor"})
		return
	}

	resources, err := query.ActorResourceList(ctx, r.authzedClient, actor.urn, resourceURN, scope, "")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error(), "message": "failed to get the resources"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"ids": resources})
}

func (r *Router) checkScope(c *gin.Context) {
	resourceURN := c.Param("urn")
	scope := c.Param("scope")

	ctx, span := tracer.Start(c.Request.Context(), "api.checkScope")
	defer span.End()

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

	actorResource, err := query.NewResourceFromURN(actor.urn)
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
