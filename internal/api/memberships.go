package api

import (
	"net/http"

	"go.infratographer.com/permissions-api/internal/types"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.infratographer.com/x/urnx"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

func (r *Router) membershipCreate(c *gin.Context) {
	roleIDStr := c.Param("role_id")

	roleID, err := uuid.Parse(roleIDStr)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"message": "not found"})
		return
	}

	_, span := tracer.Start(c.Request.Context(), "api.membershipCreate", trace.WithAttributes(attribute.String("role_id", roleIDStr)))
	defer span.End()

	var reqBody createMembershipRequest
	err = c.BindJSON(&reqBody)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "error parsing request body", "error": err.Error()})
		return
	}

	subjURN, err := urnx.Parse(reqBody.SubjectURN)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "error parsing subject URN", "error": err.Error()})
		return
	}

	subjResource, err := r.engine.NewResourceFromURN(subjURN)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "error creating resource", "error": err.Error()})
		return
	}

	role := types.Role{
		ID: roleID,
	}

	_, err = r.engine.AssignSubjectRole(c.Request.Context(), subjResource, role)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "error creating resource", "error": err.Error()})
		return
	}

	resp := createMembershipResponse{
		Success: true,
	}

	c.JSON(http.StatusCreated, resp)
}
