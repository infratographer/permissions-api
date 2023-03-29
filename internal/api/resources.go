package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
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
		c.JSON(http.StatusBadRequest, gin.H{"message": "error parsing resource URN", "error": err.Error()})
		return
	}

	resource, err := r.engine.NewResourceFromURN(resourceURN)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "error processing resource URN", "error": err.Error()})
		return
	}

	subject, err := currentSubject(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error(), "message": "failed to get the subject"})
		return
	}

	subjectResource, err := r.engine.NewResourceFromURN(subject)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "error processing subject URN", "error": err.Error()})
		return
	}

	if resource.Type != "tenant" {
		c.JSON(http.StatusBadRequest, gin.H{"message": "failed to create relationship", "error": "only tenants can be created"})
		return
	}

	roles, _, err := r.engine.CreateBuiltInRoles(ctx, resource)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "failed to create relationship", "error": err.Error()})
		return
	}

	var zedToken string
	for _, role := range roles {
		zedToken, err = r.engine.AssignSubjectRole(ctx, subjectResource, role)

		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"message": "failed to create relationship", "error": err.Error()})
			return
		}
	}

	c.JSON(http.StatusCreated, gin.H{"token": zedToken})
}

func (r *Router) resourceDelete(c *gin.Context) {
	resourceURN := c.Param("urn")

	_, span := tracer.Start(c.Request.Context(), "api.resourceDelete", trace.WithAttributes(attribute.String("urn", resourceURN)))
	defer span.End()

	c.JSON(http.StatusInternalServerError, gin.H{"message": "not implemented"})
}
