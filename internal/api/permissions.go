package api

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"go.infratographer.com/permissions-api/internal/query"
	"go.infratographer.com/x/urnx"
)

func (r *Router) checkAction(c *gin.Context) {
	resourceURNStr := c.Param("urn")
	action := c.Param("action")

	ctx, span := tracer.Start(c.Request.Context(), "api.checkAction")
	defer span.End()

	resourceURN, err := urnx.Parse(resourceURNStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "error processing resource URN", "error": err.Error()})
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

	err = r.engine.SubjectHasPermission(ctx, subjectResource, action, resource, "")
	if err != nil {
		if errors.Is(err, query.ErrActionNotAssigned) {
			c.JSON(http.StatusForbidden, gin.H{"message": "subject does not have requested action"})
			return
		}

		c.JSON(http.StatusInternalServerError, gin.H{"message": "an error occurred checking permissions", "error": err.Error()})

		return
	}

	c.JSON(http.StatusOK, gin.H{})
}
