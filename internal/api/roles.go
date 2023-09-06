package api

import (
	"net/http"

	"github.com/labstack/echo/v4"
	"go.infratographer.com/x/gidx"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

const (
	actionRoleCreate = "role_create"
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

	subjectResource, err := r.currentSubject(c)
	if err != nil {
		return err
	}

	resource, err := r.engine.NewResourceFromID(resourceID)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "error creating resource").SetInternal(err)
	}

	if err := r.checkActionWithResponse(ctx, subjectResource, actionRoleCreate, resource); err != nil {
		return err
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

func (r *Router) roleGet(c echo.Context) error {
	roleIDStr := c.Param("role_id")

	ctx, span := tracer.Start(c.Request().Context(), "api.roleGet", trace.WithAttributes(attribute.String("id", roleIDStr)))
	defer span.End()

	roleResourceID, err := gidx.Parse(roleIDStr)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "error getting resource").SetInternal(err)
	}

	roleResource, err := r.engine.NewResourceFromID(roleResourceID)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "error getting resource").SetInternal(err)
	}

	role, err := r.engine.GetRole(ctx, roleResource, "")
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "error getting resource").SetInternal(err)
	}

	resp := roleResponse{
		ID:      role.ID,
		Actions: role.Actions,
	}

	return c.JSON(http.StatusOK, resp)
}

func (r *Router) rolesList(c echo.Context) error {
	resourceIDStr := c.Param("id")

	ctx, span := tracer.Start(c.Request().Context(), "api.rolesList", trace.WithAttributes(attribute.String("id", resourceIDStr)))
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

func (r *Router) roleDelete(c echo.Context) error {
	roleIDStr := c.Param("id")

	ctx, span := tracer.Start(c.Request().Context(), "api.roleDelete", trace.WithAttributes(attribute.String("id", roleIDStr)))
	defer span.End()

	roleResourceID, err := gidx.Parse(roleIDStr)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "error deleting resource").SetInternal(err)
	}

	roleResource, err := r.engine.NewResourceFromID(roleResourceID)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "error deleting resource").SetInternal(err)
	}

	_, err = r.engine.DeleteRole(ctx, roleResource, "")
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "error deleting resource").SetInternal(err)
	}

	resp := deleteRoleResponse{
		Success: true,
	}

	return c.JSON(http.StatusOK, resp)
}

func (r *Router) roleGetResource(c echo.Context) error {
	roleIDStr := c.Param("role_id")

	ctx, span := tracer.Start(c.Request().Context(), "api.roleGetResource", trace.WithAttributes(attribute.String("id", roleIDStr)))
	defer span.End()

	roleResourceID, err := gidx.Parse(roleIDStr)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "error getting resource").SetInternal(err)
	}

	roleResource, err := r.engine.NewResourceFromID(roleResourceID)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "error getting resource").SetInternal(err)
	}

	resource, err := r.engine.GetRoleResource(ctx, roleResource, "")
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "error getting resource").SetInternal(err)
	}

	resp := resourceResponse{
		ID: resource.ID,
	}

	return c.JSON(http.StatusOK, resp)
}
