package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

func (r *Router) healthz(c *gin.Context) {
	var out struct {
		Success bool `json:"success"`
	}

	out.Success = true

	c.JSON(http.StatusOK, out)
}
