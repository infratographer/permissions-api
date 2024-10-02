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

	"go.infratographer.com/permissions-api/internal/iapl"
	"go.infratographer.com/permissions-api/internal/query"
	"go.infratographer.com/permissions-api/internal/storage"
	"go.infratographer.com/permissions-api/internal/types"
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

	if err := r.checkActionWithResponse(ctx, subjectResource, string(iapl.RoleActionCreate), resource); err != nil {
		return err
	}

	role, err := r.engine.CreateRole(
		ctx, subjectResource, resource, reqBody.Manager,
		strings.TrimSpace(reqBody.Name), reqBody.Actions,
	)

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
		Manager:    role.Manager,
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

	switch {
	case err == nil:
	case errors.Is(err, query.ErrRoleNotFound):
		return echo.NewHTTPError(http.StatusNotFound, "resource not found").SetInternal(err)
	default:
		return echo.NewHTTPError(http.StatusInternalServerError, "error getting resource").SetInternal(err)
	}

	if err := r.checkActionWithResponse(ctx, subjectResource, string(iapl.RoleActionUpdate), resource); err != nil {
		return err
	}

	role, err := r.engine.UpdateRole(
		ctx, subjectResource, roleResource,
		strings.TrimSpace(reqBody.Name), reqBody.Actions,
	)

	switch {
	case err == nil:
	case errors.Is(err, query.ErrInvalidAction):
		return echo.NewHTTPError(http.StatusBadRequest, "error updating resource: "+err.Error())
	case errors.Is(err, storage.ErrRoleNameTaken):
		return echo.NewHTTPError(http.StatusConflict, "error updating resource: "+err.Error())
	default:
		return echo.NewHTTPError(http.StatusInternalServerError, "error updating resource").SetInternal(err)
	}

	resp := roleResponse{
		ID:         role.ID,
		Name:       role.Name,
		Manager:    role.Manager,
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

	switch {
	case err == nil:
	case errors.Is(err, query.ErrRoleNotFound):
		return echo.NewHTTPError(http.StatusNotFound, "resource not found").SetInternal(err)
	default:
		return echo.NewHTTPError(http.StatusInternalServerError, "error getting resource").SetInternal(err)
	}

	// TODO: This shows an error for the role's resource, not the role. Determine if that
	// matters.
	if err := r.checkActionWithResponse(ctx, subjectResource, string(iapl.RoleActionGet), resource); err != nil {
		return err
	}

	role, err := r.engine.GetRole(ctx, roleResource)

	switch {
	case err == nil:
	case errors.Is(err, query.ErrRoleNotFound):
		return echo.NewHTTPError(http.StatusNotFound, "role not found").SetInternal(err)
	default:
		return echo.NewHTTPError(http.StatusInternalServerError, "error getting role").SetInternal(err)
	}

	resp := roleResponse{
		ID:         role.ID,
		Name:       role.Name,
		Manager:    role.Manager,
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

	if err := r.checkActionWithResponse(ctx, subjectResource, string(iapl.RoleActionList), resource); err != nil {
		return err
	}

	var roles []types.Role

	params := c.QueryParams()

	if params.Has("manager") {
		roles, err = r.engine.ListManagerRoles(ctx, params.Get("manager"), resource)
	} else {
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
			Manager:   role.Manager,
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

	switch {
	case err == nil:
	case errors.Is(err, query.ErrRoleNotFound):
		return echo.NewHTTPError(http.StatusNotFound, "resource not found").SetInternal(err)
	default:
		return echo.NewHTTPError(http.StatusInternalServerError, "error getting resource").SetInternal(err)
	}

	if err := r.checkActionWithResponse(ctx, subjectResource, string(iapl.RoleActionDelete), resource); err != nil {
		return err
	}

	err = r.engine.DeleteRole(ctx, roleResource)

	switch {
	case err == nil:
	case errors.Is(err, query.ErrRoleNotFound):
		return echo.NewHTTPError(http.StatusNotFound, "role not found").SetInternal(err)
	default:
		return echo.NewHTTPError(http.StatusInternalServerError, "error deleting role").SetInternal(err)
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

	switch {
	case err == nil:
	case errors.Is(err, query.ErrRoleNotFound):
		return echo.NewHTTPError(http.StatusNotFound, "role not found").SetInternal(err)
	default:
		return echo.NewHTTPError(http.StatusInternalServerError, "error getting role").SetInternal(err)
	}

	if err := r.checkActionWithResponse(ctx, subjectResource, string(iapl.RoleActionGet), resource); err != nil {
		return err
	}

	resp := resourceResponse{
		ID: resource.ID,
	}

	return c.JSON(http.StatusOK, resp)
}
