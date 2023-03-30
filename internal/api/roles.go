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

	_, span := tracer.Start(c.Request.Context(), "api.roleCreate", trace.WithAttributes(attribute.String("urn", resourceURNStr)))
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

	resp := roleResponse{
		ID:      role.ID,
		Actions: role.Actions,
	}

	c.JSON(http.StatusCreated, resp)
}

func (r *Router) rolesList(c *gin.Context) {
	resourceURNStr := c.Param("urn")

	_, span := tracer.Start(c.Request.Context(), "api.roleGet", trace.WithAttributes(attribute.String("urn", resourceURNStr)))
	defer span.End()

	resourceURN, err := urnx.Parse(resourceURNStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "error parsing resource URN", "error": err.Error()})
		return
	}

	resource, err := r.engine.NewResourceFromURN(resourceURN)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "error creating resource", "error": err.Error()})
		return
	}

	roles, err := r.engine.ListRoles(c.Request.Context(), resource, "")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "error getting role", "error": err.Error()})
		return
	}

	resp := listRolesResponse{
		Data: []roleResponse{},
	}

	for _, role := range roles {
		roleResp := roleResponse{
			ID:      role.ID,
			Actions: role.Actions,
		}

		resp.Data = append(resp.Data, roleResp)
	}

	c.JSON(http.StatusOK, resp)
}
