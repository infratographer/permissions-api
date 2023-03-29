// Package api defines the server for permissions-api
package api

import (
	"github.com/gin-gonic/gin"
	"go.hollow.sh/toolbox/ginjwt"
)

func newAuthMiddleware(cfg ginjwt.AuthConfig) (func(*gin.Context), error) {
	mw, err := ginjwt.NewMultiTokenMiddlewareFromConfigs(cfg)
	if err != nil {
		return nil, err
	}

	return mw.AuthRequired(nil), nil
}
