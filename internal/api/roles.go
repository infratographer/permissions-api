package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"go.infratographer.com/x/urnx"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

func (r *Router) roleCreate(c *gin.Context) {
	resourceURNStr := c.Param("urn")

	_, span := tracer.Start(c.Request.Context(), "api.resourceCreate", trace.WithAttributes(attribute.String("urn", resourceURNStr)))
	defer span.End()

	resourceURN, err := urnx.Parse(resourceURNStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "error parsing resource URN", "error": err.Error()})
		return
	}

	var reqBody createRoleRequest
	err = c.BindJSON(&reqBody)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "error parsing request body", "error": err.Error()})
		return
	}

	resource, err := r.engine.NewResourceFromURN(resourceURN)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "error creating resource", "error": err.Error()})
		return
	}

	role, _, err := r.engine.CreateRole(c.Request.Context(), resource, reqBody.Actions)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "error creating resource", "error": err.Error()})
		return
	}

	resp := createRoleResponse{
		ID:      role.ID,
		Actions: role.Actions,
	}

	c.JSON(http.StatusCreated, resp)
}
