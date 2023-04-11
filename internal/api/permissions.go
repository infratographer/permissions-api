package api

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"go.infratographer.com/permissions-api/internal/query"
	"go.infratographer.com/x/urnx"
)

// checkAction will check if a subject is allowed to perform an action on a resource
// scoped to the tenant.
// This is the permissions check endpoint.
// It will return a 200 if the subject is allowed to perform the action on the resource.
// It will return a 403 if the subject is not allowed to perform the action on the resource.
//
// Note that this expects a JWT token to be present in the request. This token must
// contain the subject of the request in the "sub" claim.
//
// The following query parameters are required:
// - resource: the resource URN to check
// - tenant: the tenant URN to check
// - action: the action to check
func (r *Router) checkAction(c *gin.Context) {
	ctx, span := tracer.Start(c.Request.Context(), "api.checkAction")
	defer span.End()

	// Get the query parameters. These are mandatory.
	resourceURNStr, hasQuery := c.GetQuery("resource")
	if !hasQuery {
		c.JSON(http.StatusBadRequest, gin.H{"message": "missing resource query parameter"})
		return
	}

	tenantURNStr, hasQuery := c.GetQuery("tenant")
	if !hasQuery {
		c.JSON(http.StatusBadRequest, gin.H{"message": "missing tenant query parameter"})
		return
	}

	action, hasQuery := c.GetQuery("action")
	if !hasQuery {
		c.JSON(http.StatusBadRequest, gin.H{"message": "missing action query parameter"})
		return
	}

	// Query parameter validation
	// Note that we currently only check the tenant as a scope. The
	// resource is not checked as of yet.
	_, err := urnx.Parse(resourceURNStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "error processing resource URN", "error": err.Error()})
		return
	}

	tenantURN, err := urnx.Parse(tenantURNStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "error processing tenant URN", "error": err.Error()})
		return
	}

	tenantResource, err := r.engine.NewResourceFromURN(tenantURN)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "error processing tenant resource URN", "error": err.Error()})
		return
	}

	// Subject validation
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

	// Check the permissions
	err = r.engine.SubjectHasPermission(ctx, subjectResource, action, tenantResource, "")
	if err != nil && errors.Is(err, query.ErrActionNotAssigned) {
		msg := fmt.Sprintf("subject '%s' does not have permission to perform action '%s' on resource '%s' scoped on tenant '%s'",
			subject, action, resourceURNStr, tenantURNStr)
		c.JSON(http.StatusForbidden, gin.H{"message": msg})

		return
	} else if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "an error occurred checking permissions", "error": err.Error()})

		return
	}

	c.JSON(http.StatusOK, gin.H{})
}
