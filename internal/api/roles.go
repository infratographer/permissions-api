package api

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/labstack/echo/v4"
	"go.infratographer.com/x/gidx"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"

	"go.infratographer.com/permissions-api/internal/query"
	"go.infratographer.com/permissions-api/internal/storage"
	"go.infratographer.com/permissions-api/internal/types"
)

const (
	actionRoleCreate = "role_create"
	actionRoleGet    = "role_get"
	actionRoleList   = "role_list"
	actionRoleUpdate = "role_update"
	actionRoleDelete = "role_delete"
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

	apiversion := r.getAPIVersion(c)

	var role types.Role

	switch apiversion {
	case "v2":
		role, err = r.engine.CreateRoleV2(ctx, subjectResource, resource, reqBody.Name, reqBody.Actions)
	default:
		role, err = r.engine.CreateRole(ctx, subjectResource, resource, reqBody.Name, reqBody.Actions)
	}

	if err != nil {
		switch {
		case errors.Is(err, storage.ErrRoleNameTaken), strings.Contains(err.Error(), " InvalidArgument"):
			return echo.NewHTTPError(http.StatusBadRequest, "error creating resource: "+err.Error()).SetInternal(err)
		default:
			return echo.NewHTTPError(http.StatusInternalServerError, "error creating resource").SetInternal(err)
		}
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

func (r *Router) roleUpdate(c echo.Context) error {
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

	roleResource, err := r.engine.NewResourceFromID(roleID)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "error updating role").SetInternal(err)
	}

	// Roles belong to resources by way of the actions they can perform; do the permissions
	// check on the role resource.
	resource, err := r.engine.GetRoleResource(ctx, roleResource)
	if err != nil {
		if errors.Is(err, query.ErrRoleNotFound) {
			return echo.NewHTTPError(http.StatusNotFound, "resource not found").SetInternal(err)
		}

		return echo.NewHTTPError(http.StatusInternalServerError, "error getting resource").SetInternal(err)
	}

	if err := r.checkActionWithResponse(ctx, subjectResource, actionRoleUpdate, resource); err != nil {
		return err
	}

	role, err := r.engine.UpdateRole(ctx, subjectResource, roleResource, reqBody.Name, reqBody.Actions)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "error updating resource").SetInternal(err)
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

func (r *Router) roleGet(c echo.Context) error {
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

	roleResource, err := r.engine.NewResourceFromID(roleResourceID)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "error getting resource").SetInternal(err)
	}

	// Roles belong to resources by way of the actions they can perform; do the permissions
	// check on the role resource.
	resource, err := r.engine.GetRoleResource(ctx, roleResource)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "error getting resource").SetInternal(err)
	}

	// TODO: This shows an error for the role's resource, not the role. Determine if that
	// matters.
	if err := r.checkActionWithResponse(ctx, subjectResource, actionRoleGet, resource); err != nil {
		return err
	}

	role, err := r.engine.GetRole(ctx, roleResource)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "error getting resource").SetInternal(err)
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

func (r *Router) rolesList(c echo.Context) error {
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

	apiversion := r.getAPIVersion(c)

	var roles []types.Role

	switch apiversion {
	case "v2":
		roles, err = r.engine.ListRolesV2(ctx, resource)
	default:
		roles, err = r.engine.ListRoles(ctx, resource)
	}

	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "error getting role").SetInternal(err)
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

func (r *Router) roleDelete(c echo.Context) error {
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

	roleResource, err := r.engine.NewResourceFromID(roleResourceID)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "error deleting resource").SetInternal(err)
	}

	// Roles belong to resources by way of the actions they can perform; do the permissions
	// check on the role resource.
	resource, err := r.engine.GetRoleResource(ctx, roleResource)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "error getting resource").SetInternal(err)
	}

	if err := r.checkActionWithResponse(ctx, subjectResource, actionRoleDelete, resource); err != nil {
		return err
	}

	if err = r.engine.DeleteRole(ctx, roleResource); err != nil {
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

	subjectResource, err := r.currentSubject(c)
	if err != nil {
		return err
	}

	roleResource, err := r.engine.NewResourceFromID(roleResourceID)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "error getting resource").SetInternal(err)
	}

	// There's a little irony here in that getting a role's resource here is required to actually
	// do the permissions check.
	resource, err := r.engine.GetRoleResource(ctx, roleResource)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "error getting resource").SetInternal(err)
	}

	if err := r.checkActionWithResponse(ctx, subjectResource, actionRoleGet, resource); err != nil {
		return err
	}

	resp := resourceResponse{
		ID: resource.ID,
	}

	return c.JSON(http.StatusOK, resp)
}
