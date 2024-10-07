package api

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"go.infratographer.com/permissions-api/internal/iapl"
	"go.infratographer.com/permissions-api/internal/types"

	"github.com/labstack/echo/v4"
	"go.infratographer.com/x/gidx"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

func (r *Router) roleV2Create(c echo.Context) error {
	resourceIDStr := c.Param("id")

	ctx, span := tracer.Start(c.Request().Context(), "api.roleV2Create", trace.WithAttributes(attribute.String("id", resourceIDStr)))
	defer span.End()

	resourceID, err := gidx.Parse(resourceIDStr)
	if err != nil {
		return r.errorResponse("error parsing resource ID", fmt.Errorf("%w: %s", ErrInvalidID, err.Error()))
	}

	var reqBody createRoleRequest

	err = c.Bind(&reqBody)
	if err != nil {
		return r.errorResponse(err.Error(), ErrParsingRequestBody)
	}

	subjectResource, err := r.currentSubject(c)
	if err != nil {
		return err
	}

	resource, err := r.engine.NewResourceFromID(resourceID)
	if err != nil {
		return r.errorResponse("error creating resource", err)
	}

	if err := r.checkActionWithResponse(ctx, subjectResource, string(iapl.RoleActionCreate), resource); err != nil {
		return err
	}

	role, err := r.engine.CreateRoleV2(
		ctx, subjectResource, resource, reqBody.Manager,
		strings.TrimSpace(reqBody.Name), reqBody.Actions,
	)
	if err != nil {
		return r.errorResponse("error creating resource", err)
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

func (r *Router) roleV2Update(c echo.Context) error {
	roleIDStr := c.Param("role_id")

	ctx, span := tracer.Start(c.Request().Context(), "api.roleV2Update", trace.WithAttributes(attribute.String("id", roleIDStr)))
	defer span.End()

	roleID, err := gidx.Parse(roleIDStr)
	if err != nil {
		return r.errorResponse("error parsing resource ID", fmt.Errorf("%w: %s", ErrInvalidID, err.Error()))
	}

	var reqBody updateRoleRequest

	err = c.Bind(&reqBody)
	if err != nil {
		return r.errorResponse(err.Error(), ErrParsingRequestBody)
	}

	subjectResource, err := r.currentSubject(c)
	if err != nil {
		return err
	}

	// Roles themselves are the resource, permissions check should be performed
	// on the roles themselves.
	roleResource, err := r.engine.NewResourceFromID(roleID)
	if err != nil {
		return r.errorResponse("error creating resource", err)
	}

	if err := r.checkActionWithResponse(ctx, subjectResource, string(iapl.RoleActionUpdate), roleResource); err != nil {
		return err
	}

	role, err := r.engine.UpdateRoleV2(
		ctx, subjectResource, roleResource,
		strings.TrimSpace(reqBody.Name), reqBody.Actions,
	)
	if err != nil {
		return r.errorResponse("error updating role", err)
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

func (r *Router) roleV2Get(c echo.Context) error {
	roleIDStr := c.Param("role_id")

	ctx, span := tracer.Start(c.Request().Context(), "api.roleV2Get", trace.WithAttributes(attribute.String("id", roleIDStr)))
	defer span.End()

	roleResourceID, err := gidx.Parse(roleIDStr)
	if err != nil {
		return r.errorResponse("error parsing resource ID", fmt.Errorf("%w: %s", ErrInvalidID, err.Error()))
	}

	subjectResource, err := r.currentSubject(c)
	if err != nil {
		return err
	}

	// Roles themselves are the resource, permissions check should be performed
	// on the roles themselves.
	roleResource, err := r.engine.NewResourceFromID(roleResourceID)
	if err != nil {
		return r.errorResponse("error creating resource", err)
	}

	if err := r.checkActionWithResponse(ctx, subjectResource, string(iapl.RoleActionGet), roleResource); err != nil {
		return err
	}

	role, err := r.engine.GetRoleV2(ctx, roleResource)
	if err != nil {
		return r.errorResponse("error getting role", err)
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

func (r *Router) roleV2sList(c echo.Context) error {
	resourceIDStr := c.Param("id")

	ctx, span := tracer.Start(c.Request().Context(), "api.roleV2sList", trace.WithAttributes(attribute.String("id", resourceIDStr)))
	defer span.End()

	resourceID, err := gidx.Parse(resourceIDStr)
	if err != nil {
		return r.errorResponse("error parsing resource ID", fmt.Errorf("%w: %s", ErrInvalidID, err.Error()))
	}

	subjectResource, err := r.currentSubject(c)
	if err != nil {
		return err
	}

	resource, err := r.engine.NewResourceFromID(resourceID)
	if err != nil {
		return r.errorResponse("error creating resource", err)
	}

	if err := r.checkActionWithResponse(ctx, subjectResource, string(iapl.RoleActionList), resource); err != nil {
		return err
	}

	var roles []types.Role

	params := c.QueryParams()

	if params.Has("manager") {
		roles, err = r.engine.ListManagerRolesV2(ctx, params.Get("manager"), resource)
	} else {
		roles, err = r.engine.ListRolesV2(ctx, resource)
	}

	if err != nil {
		return r.errorResponse("error getting roles", err)
	}

	resp := listRolesV2Response{
		Data: []listRolesV2Role{},
	}

	for _, role := range roles {
		roleResp := listRolesV2Role{
			ID:         role.ID,
			Name:       role.Name,
			Manager:    role.Manager,
			ResourceID: role.ResourceID,
			CreatedBy:  role.CreatedBy,
			UpdatedBy:  role.UpdatedBy,
			CreatedAt:  role.CreatedAt.Format(time.RFC3339),
			UpdatedAt:  role.UpdatedAt.Format(time.RFC3339),
		}

		resp.Data = append(resp.Data, roleResp)
	}

	return c.JSON(http.StatusOK, resp)
}

func (r *Router) roleV2Delete(c echo.Context) error {
	roleIDStr := c.Param("id")

	ctx, span := tracer.Start(c.Request().Context(), "api.roleV2Delete", trace.WithAttributes(attribute.String("id", roleIDStr)))
	defer span.End()

	roleResourceID, err := gidx.Parse(roleIDStr)
	if err != nil {
		return r.errorResponse("error parsing resource ID", fmt.Errorf("%w: %s", ErrInvalidID, err.Error()))
	}

	subjectResource, err := r.currentSubject(c)
	if err != nil {
		return err
	}

	// Roles themselves are the resource, permissions check should be performed
	// on the roles themselves.
	roleResource, err := r.engine.NewResourceFromID(roleResourceID)
	if err != nil {
		return r.errorResponse("error creating resource", err)
	}

	if err := r.checkActionWithResponse(ctx, subjectResource, string(iapl.RoleActionDelete), roleResource); err != nil {
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
