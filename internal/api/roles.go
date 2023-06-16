package api

import (
	"net/http"

	"github.com/labstack/echo/v4"
	"go.infratographer.com/x/gidx"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

func (r *Router) roleCreate(c echo.Context) error {
	resourceIDStr := c.Param("id")

	ctx, span := tracer.Start(c.Request().Context(), "api.roleCreate", trace.WithAttributes(attribute.String("id", resourceIDStr)))
	defer span.End()

	resourceID, err := gidx.Parse(resourceIDStr)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "error parsing resource ID").SetInternal(err)
	}

	var reqBody createRoleRequest

	err = c.Bind(&reqBody)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "error parsing request body").SetInternal(err)
	}

	resource, err := r.engine.NewResourceFromID(resourceID)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "error creating resource").SetInternal(err)
	}

	role, _, err := r.engine.CreateRole(ctx, resource, reqBody.Actions)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "error creating resource").SetInternal(err)
	}

	resp := roleResponse{
		ID:      role.ID,
		Actions: role.Actions,
	}

	return c.JSON(http.StatusCreated, resp)
}

func (r *Router) rolesList(c echo.Context) error {
	resourceIDStr := c.Param("id")

	ctx, span := tracer.Start(c.Request().Context(), "api.roleGet", trace.WithAttributes(attribute.String("id", resourceIDStr)))
	defer span.End()

	resourceID, err := gidx.Parse(resourceIDStr)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "error parsing resource ID").SetInternal(err)
	}

	resource, err := r.engine.NewResourceFromID(resourceID)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "error creating resource").SetInternal(err)
	}

	roles, err := r.engine.ListRoles(ctx, resource, "")
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "error getting role").SetInternal(err)
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

	return c.JSON(http.StatusOK, resp)
}
