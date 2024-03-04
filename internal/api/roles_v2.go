package api

import (
	"errors"
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
	"go.infratographer.com/x/gidx"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"

	"go.infratographer.com/permissions-api/internal/query"
	"go.infratographer.com/permissions-api/internal/storage"
)

func (r *Router) roleV2Create(c echo.Context) error {
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

	role, err := r.engine.CreateRoleV2(ctx, subjectResource, resource, reqBody.Name, reqBody.Actions)

	switch {
	case err == nil:
	case errors.Is(err, query.ErrInvalidAction):
		return echo.NewHTTPError(http.StatusBadRequest, "error creating resource: "+err.Error())
	case errors.Is(err, storage.ErrRoleAlreadyExists), errors.Is(err, storage.ErrRoleNameTaken):
		return echo.NewHTTPError(http.StatusConflict, "error creating resource: "+err.Error())
	default:
		return echo.NewHTTPError(http.StatusInternalServerError, "error creating resource").SetInternal(err)
	}

	resp := roleResponse{
		ID:         role.ID,
		Name:       role.Name,
		Actions:    role.Actions,
		ResourceID: role.ResourceID,
		CreatedBy:  role.CreatedBy,
		UpdatedBy:  role.UpdatedBy,
		CreatedAt:  role.CreatedAt.Format(time.RFC3339),
		UpdatedAt:  role.UpdatedAt.Format(time.RFC3339),
	}

	return c.JSON(http.StatusCreated, resp)
}

func (r *Router) roleV2Update(c echo.Context) error {
	roleIDStr := c.Param("role_id")

	ctx, span := tracer.Start(c.Request().Context(), "api.roleUpdate", trace.WithAttributes(attribute.String("id", roleIDStr)))
	defer span.End()

	roleID, err := gidx.Parse(roleIDStr)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "error parsing role ID").SetInternal(err)
	}

	var reqBody updateRoleRequest

	err = c.Bind(&reqBody)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "error parsing request body").SetInternal(err)
	}

	subjectResource, err := r.currentSubject(c)
	if err != nil {
		return err
	}

	// Roles themselves are the resource, permissions check should be performed
	// on the roles themselves.
	roleResource, err := r.engine.NewResourceFromID(roleID)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "error getting resource").SetInternal(err)
	}

	// TODO: This shows an error for the role's resource, not the role. Determine if that
	// matters.
	if err := r.checkActionWithResponse(ctx, subjectResource, actionRoleGet, roleResource); err != nil {
		return err
	}

	role, err := r.engine.UpdateRoleV2(ctx, subjectResource, roleResource, reqBody.Name, reqBody.Actions)
	if err != nil {
		return r.errorResponse("error updating role", err)
	}

	resp := roleResponse{
		ID:         role.ID,
		Name:       role.Name,
		Actions:    role.Actions,
		ResourceID: role.ResourceID,
		CreatedBy:  role.CreatedBy,
		UpdatedBy:  role.UpdatedBy,
		CreatedAt:  role.CreatedAt.Format(time.RFC3339),
		UpdatedAt:  role.UpdatedAt.Format(time.RFC3339),
	}

	return c.JSON(http.StatusOK, resp)
}

func (r *Router) roleV2Get(c echo.Context) error {
	roleIDStr := c.Param("role_id")

	ctx, span := tracer.Start(c.Request().Context(), "api.roleGet", trace.WithAttributes(attribute.String("id", roleIDStr)))
	defer span.End()

	roleResourceID, err := gidx.Parse(roleIDStr)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "error getting resource").SetInternal(err)
	}

	subjectResource, err := r.currentSubject(c)
	if err != nil {
		return err
	}

	// Roles themselves are the resource, permissions check should be performed
	// on the roles themselves.
	roleResource, err := r.engine.NewResourceFromID(roleResourceID)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "error getting resource").SetInternal(err)
	}

	// TODO: This shows an error for the role's resource, not the role. Determine if that
	// matters.
	if err := r.checkActionWithResponse(ctx, subjectResource, actionRoleGet, roleResource); err != nil {
		return err
	}

	role, err := r.engine.GetRoleV2(ctx, roleResource)
	if err != nil {
		return r.errorResponse("error getting role", err)
	}

	resp := roleResponse{
		ID:         role.ID,
		Name:       role.Name,
		Actions:    role.Actions,
		ResourceID: role.ResourceID,
		CreatedBy:  role.CreatedBy,
		UpdatedBy:  role.UpdatedBy,
		CreatedAt:  role.CreatedAt.Format(time.RFC3339),
		UpdatedAt:  role.UpdatedAt.Format(time.RFC3339),
	}

	return c.JSON(http.StatusOK, resp)
}

func (r *Router) roleV2sList(c echo.Context) error {
	resourceIDStr := c.Param("id")

	ctx, span := tracer.Start(c.Request().Context(), "api.rolesList", trace.WithAttributes(attribute.String("id", resourceIDStr)))
	defer span.End()

	resourceID, err := gidx.Parse(resourceIDStr)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "error parsing resource ID").SetInternal(err)
	}

	subjectResource, err := r.currentSubject(c)
	if err != nil {
		return err
	}

	resource, err := r.engine.NewResourceFromID(resourceID)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "error creating resource").SetInternal(err)
	}

	if err := r.checkActionWithResponse(ctx, subjectResource, actionRoleList, resource); err != nil {
		return err
	}

	roles, err := r.engine.ListRolesV2(ctx, resource)
	if err != nil {
		return r.errorResponse("error getting roles", err)
	}

	resp := listRolesResponse{
		Data: []roleResponse{},
	}

	for _, role := range roles {
		roleResp := roleResponse{
			ID:        role.ID,
			Name:      role.Name,
			Actions:   role.Actions,
			CreatedBy: role.CreatedBy,
			UpdatedBy: role.UpdatedBy,
			CreatedAt: role.CreatedAt.Format(time.RFC3339),
			UpdatedAt: role.UpdatedAt.Format(time.RFC3339),
		}

		resp.Data = append(resp.Data, roleResp)
	}

	return c.JSON(http.StatusOK, resp)
}

func (r *Router) roleV2Delete(c echo.Context) error {
	roleIDStr := c.Param("id")

	ctx, span := tracer.Start(c.Request().Context(), "api.roleDelete", trace.WithAttributes(attribute.String("id", roleIDStr)))
	defer span.End()

	roleResourceID, err := gidx.Parse(roleIDStr)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "error deleting resource").SetInternal(err)
	}

	subjectResource, err := r.currentSubject(c)
	if err != nil {
		return err
	}

	// Roles themselves are the resource, permissions check should be performed
	// on the roles themselves.
	roleResource, err := r.engine.NewResourceFromID(roleResourceID)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "error getting resource").SetInternal(err)
	}

	if err := r.checkActionWithResponse(ctx, subjectResource, actionRoleDelete, roleResource); err != nil {
		return err
	}

	err = r.engine.DeleteRoleV2(ctx, roleResource)
	if err != nil {
		return r.errorResponse("error deleting role", err)
	}

	resp := deleteRoleResponse{
		Success: true,
	}

	return c.JSON(http.StatusOK, resp)
}

func (r *Router) listActions(c echo.Context) error {
	return c.JSON(http.StatusOK, r.engine.AllActions())
}
