package api

import (
	"net/http"

	"github.com/labstack/echo/v4"
	"go.infratographer.com/x/gidx"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"

	"go.infratographer.com/permissions-api/internal/types"
)

const (
	actionRoleBindingCreate = "rolebinding_create"
	actionRoleBindingList   = "rolebinding_list"
	actionRoleBindingDelete = "rolebinding_delete"
)

func resourceToSubject(subjects []types.RoleBindingSubject) []roleBindingSubject {
	resp := make([]roleBindingSubject, len(subjects))
	for i, subj := range subjects {
		resp[i] = roleBindingSubject{
			ID:        subj.SubjectResource.ID,
			Type:      subj.SubjectResource.Type,
			Condition: nil,
		}
	}

	return resp
}

func (r *Router) roleBindingCreate(c echo.Context) error {
	resourceIDStr := c.Param("id")

	ctx, span := tracer.Start(
		c.Request().Context(), "api.roleBindingCreate",
		trace.WithAttributes(attribute.String("id", resourceIDStr)),
	)
	defer span.End()

	resourceID, err := gidx.Parse(resourceIDStr)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "error parsing resource ID").SetInternal(err)
	}

	var body roleBindingRequest

	err = c.Bind(&body)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "error parsing request body").SetInternal(err)
	}

	resource, err := r.engine.NewResourceFromID(resourceID)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "error creating resource").SetInternal(err)
	}

	subjectResource, err := r.currentSubject(c)
	if err != nil {
		return err
	}

	// permissions on role binding actions, similar to roles v1, are granted on the resources
	if err := r.checkActionWithResponse(ctx, subjectResource, actionRoleBindingCreate, resource); err != nil {
		return err
	}

	roleID, err := gidx.Parse(body.RoleID)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "error parsing role ID").SetInternal(err)
	}

	roleResource, err := r.engine.NewResourceFromID(roleID)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "error creating role resource").SetInternal(err)
	}

	subjects := make([]types.RoleBindingSubject, len(body.Subjects))

	for i, s := range body.Subjects {
		subj, err := r.engine.NewResourceFromID(s.ID)
		if err != nil {
			return echo.NewHTTPError(http.StatusBadRequest, "error creating subject resource").SetInternal(err)
		}

		subjects[i] = types.RoleBindingSubject{
			SubjectResource: subj,
			Condition:       nil,
		}
	}

	rolebinding, err := r.engine.BindRole(ctx, resource, roleResource, subjects)
	if err != nil {
		return r.errorResponse("error creating role-binding", err)
	}

	resp := roleBindingResponse{
		Subjects: resourceToSubject(rolebinding.Subjects),
		Role: roleBindingResponseRole{
			ID:   rolebinding.Role.ID,
			Name: rolebinding.Role.Name,
		},
	}

	return c.JSON(http.StatusOK, resp)
}

func (r *Router) roleBindingsList(c echo.Context) error {
	resourceIDStr := c.Param("id")
	roleIDStr := c.QueryParam("role_id")

	ctx, span := tracer.Start(
		c.Request().Context(), "api.roleBindingList",
		trace.WithAttributes(attribute.String("id", resourceIDStr)),
	)
	defer span.End()

	resourceID, err := gidx.Parse(resourceIDStr)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "error parsing resource ID").SetInternal(err)
	}

	resource, err := r.engine.NewResourceFromID(resourceID)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "error getting resource").SetInternal(err)
	}

	subjectResource, err := r.currentSubject(c)
	if err != nil {
		return err
	}

	if err := r.checkActionWithResponse(ctx, subjectResource, actionRoleBindingList, resource); err != nil {
		return err
	}

	roleFilter := (*types.Resource)(nil)

	if roleIDStr != "" {
		roleID, err := gidx.Parse(roleIDStr)
		if err != nil {
			return echo.NewHTTPError(http.StatusBadRequest, "error parsing role ID").SetInternal(err)
		}

		roleResource, err := r.engine.NewResourceFromID(roleID)
		if err != nil {
			return echo.NewHTTPError(http.StatusBadRequest, "error creating role resource").SetInternal(err)
		}

		roleFilter = &roleResource
	}

	rbs, err := r.engine.ListRoleBindings(ctx, resource, roleFilter)
	if err != nil {
		return r.errorResponse("error listing role-binding", err)
	}

	resp := listRoleBindingsResponse{
		Data: make([]roleBindingResponse, len(rbs)),
	}

	for i, rb := range rbs {
		resp.Data[i] = roleBindingResponse{
			Subjects: resourceToSubject(rb.Subjects),
			Role: roleBindingResponseRole{
				ID:   rb.Role.ID,
				Name: rb.Role.Name,
			},
		}
	}

	return c.JSON(http.StatusOK, resp)
}

func (r *Router) roleBindingsDelete(c echo.Context) error {
	resourceIDStr := c.Param("id")

	ctx, span := tracer.Start(
		c.Request().Context(), "api.roleBindingDelete",
		trace.WithAttributes(attribute.String("id", resourceIDStr)),
	)
	defer span.End()

	resourceID, err := gidx.Parse(resourceIDStr)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "error parsing resource ID").SetInternal(err)
	}

	var body roleBindingRequest

	err = c.Bind(&body)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "error parsing request body").SetInternal(err)
	}

	resource, err := r.engine.NewResourceFromID(resourceID)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "error creating resource").SetInternal(err)
	}

	subjectResource, err := r.currentSubject(c)
	if err != nil {
		return err
	}

	// permissions on role binding actions, similar to roles v1, are granted on the resources
	if err := r.checkActionWithResponse(ctx, subjectResource, actionRoleBindingDelete, resource); err != nil {
		return err
	}

	roleID, err := gidx.Parse(body.RoleID)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "error parsing role ID").SetInternal(err)
	}

	roleResource, err := r.engine.NewResourceFromID(roleID)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "error creating role resource").SetInternal(err)
	}

	subjects := make([]types.RoleBindingSubject, len(body.Subjects))

	for i, s := range body.Subjects {
		subj, err := r.engine.NewResourceFromID(s.ID)
		if err != nil {
			return echo.NewHTTPError(http.StatusBadRequest, "error creating subject resource").SetInternal(err)
		}

		subjects[i] = types.RoleBindingSubject{
			SubjectResource: subj,
			Condition:       nil,
		}
	}

	if err := r.engine.UnbindRole(ctx, resource, roleResource, subjects); err != nil {
		return r.errorResponse("error updating role-binding", err)
	}

	resp := deleteRoleBindingResponse{Success: true}

	return c.JSON(http.StatusOK, resp)
}
