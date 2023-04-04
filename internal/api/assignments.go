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

func (r *Router) assignmentCreate(c *gin.Context) {
	roleIDStr := c.Param("role_id")

	roleID, err := uuid.Parse(roleIDStr)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"message": "not found"})
		return
	}

	_, span := tracer.Start(c.Request.Context(), "api.assignmentCreate", trace.WithAttributes(attribute.String("role_id", roleIDStr)))
	defer span.End()

	var reqBody createAssignmentRequest

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

	resp := createAssignmentResponse{
		Success: true,
	}

	c.JSON(http.StatusCreated, resp)
}

func (r *Router) assignmentsList(c *gin.Context) {
	roleIDStr := c.Param("role_id")

	roleID, err := uuid.Parse(roleIDStr)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"message": "not found"})
		return
	}

	ctx, span := tracer.Start(c.Request.Context(), "api.assignmentCreate", trace.WithAttributes(attribute.String("role_id", roleIDStr)))
	defer span.End()

	role := types.Role{
		ID: roleID,
	}

	assignments, err := r.engine.ListAssignments(ctx, role, "")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "error listing assignments", "error": err.Error()})
		return
	}

	items := make([]assignmentItem, len(assignments))

	for i, res := range assignments {
		subjURN, err := r.engine.NewURNFromResource(res)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"message": "error listing assignments", "error": err.Error()})
			return
		}

		item := assignmentItem{
			SubjectURN: subjURN.String(),
		}

		items[i] = item
	}

	out := listAssignmentsResponse{
		Data: items,
	}

	c.JSON(http.StatusOK, out)
}
