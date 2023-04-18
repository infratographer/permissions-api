package api

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/labstack/echo/v4"
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
// - tenant: the tenant URN to check
// - action: the action to check
//
// The following query parameters are optional:
// - resource: the resource URN to check
func (r *Router) checkAction(c echo.Context) error {
	ctx, span := tracer.Start(c.Request().Context(), "api.checkAction")
	defer span.End()

	// Get the query parameters. These are mandatory.
	tenantURNStr, hasQuery := getParam(c, "tenant")
	if !hasQuery {
		return echo.NewHTTPError(http.StatusBadRequest, "missing tenant query parameter")
	}

	action, hasQuery := getParam(c, "action")
	if !hasQuery {
		return echo.NewHTTPError(http.StatusBadRequest, "missing action query parameter")
	}

	// Optional query parameters
	resourceURNStr, hasResourceParam := getParam(c, "resource")

	// Query parameter validation
	// Note that we currently only check the tenant as a scope. The
	// resource is not checked as of yet.
	tenantURN, err := urnx.Parse(tenantURNStr)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "error processing tenant URN").SetInternal(err)
	}

	tenantResource, err := r.engine.NewResourceFromURN(tenantURN)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "error processing tenant resource URN").SetInternal(err)
	}

	if hasResourceParam {
		_, err := urnx.Parse(resourceURNStr)
		if err != nil {
			return echo.NewHTTPError(http.StatusBadRequest, "error processing resource URN").SetInternal(err)
		}
	}

	// Subject validation
	subject, err := currentSubject(c)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "failed to get the subject").SetInternal(err)
	}

	subjectResource, err := r.engine.NewResourceFromURN(subject)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "error processing subject URN").SetInternal(err)
	}

	// Check the permissions
	err = r.engine.SubjectHasPermission(ctx, subjectResource, action, tenantResource, "")
	if err != nil && errors.Is(err, query.ErrActionNotAssigned) {
		msg := fmt.Sprintf("subject '%s' does not have permission to perform action '%s' on resource '%s' scoped on tenant '%s'",
			subject, action, resourceURNStr, tenantURNStr)

		return echo.NewHTTPError(http.StatusForbidden, msg).SetInternal(err)
	} else if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "an error occurred checking permissions").SetInternal(err)
	}

	return c.JSON(http.StatusOK, echo.Map{})
}

func getParam(c echo.Context, name string) (string, bool) {
	values, ok := c.QueryParams()[name]
	if !ok {
		return "", ok
	}

	if len(values) == 0 {
		return "", true
	}

	return values[0], true
}
