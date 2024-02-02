package api

import (
	"net/http"

	"github.com/labstack/echo/v4"
	"go.infratographer.com/x/gidx"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"

	"go.infratographer.com/permissions-api/internal/iapl"
	"go.infratographer.com/permissions-api/internal/types"
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
	if err := r.checkActionWithResponse(ctx, subjectResource, string(iapl.RoleBindingActionCreate), resource); err != nil {
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

	rb, err := r.engine.CreateRoleBinding(ctx, resource, roleResource, subjects)
	if err != nil {
		return r.errorResponse("error creating role-binding", err)
	}

	return c.JSON(
		http.StatusOK,
		roleBindingResponse{
			ID:       rb.ID,
			Subjects: resourceToSubject(rb.Subjects),
			Role: roleBindingResponseRole{
				ID:   rb.Role.ID,
				Name: rb.Role.Name,
			},
		},
	)
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

	if err := r.checkActionWithResponse(ctx, subjectResource, string(iapl.RoleBindingActionList), resource); err != nil {
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
			ID:       rb.ID,
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
	resID := c.Param("id")
	rbID := c.Param("rb_id")

	ctx, span := tracer.Start(
		c.Request().Context(), "api.roleBindingDelete",
		trace.WithAttributes(attribute.String("id", rbID)),
	)
	defer span.End()

	// resource
	resourceID, err := gidx.Parse(resID)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "error parsing resource ID").SetInternal(err)
	}

	resource, err := r.engine.NewResourceFromID(resourceID)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "error creating resource").SetInternal(err)
	}

	// role-binding
	rolebindingID, err := gidx.Parse(rbID)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "error parsing resource ID").SetInternal(err)
	}

	rbRes, err := r.engine.NewResourceFromID(rolebindingID)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "error creating resource").SetInternal(err)
	}

	actor, err := r.currentSubject(c)
	if err != nil {
		return err
	}

	// permissions on role binding actions, similar to roles v1, are granted on the resources
	if err := r.checkActionWithResponse(ctx, actor, string(iapl.RoleBindingActionDelete), resource); err != nil {
		return err
	}

	if err := r.engine.DeleteRoleBinding(ctx, rbRes, resource); err != nil {
		return r.errorResponse("error updating role-binding", err)
	}

	resp := deleteRoleBindingResponse{Success: true}

	return c.JSON(http.StatusOK, resp)
}

func (r *Router) roleBindingGet(c echo.Context) error {
	resID := c.Param("id")
	rbID := c.Param("rb_id")

	ctx, span := tracer.Start(
		c.Request().Context(), "api.roleBindingGet",
		trace.WithAttributes(attribute.String("id", rbID)),
	)
	defer span.End()

	// resource
	resourceID, err := gidx.Parse(resID)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "error parsing resource ID").SetInternal(err)
	}

	resource, err := r.engine.NewResourceFromID(resourceID)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "error creating resource").SetInternal(err)
	}

	// role-binding
	rolebindingID, err := gidx.Parse(rbID)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "error parsing resource ID").SetInternal(err)
	}

	rbRes, err := r.engine.NewResourceFromID(rolebindingID)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "error creating resource").SetInternal(err)
	}

	actor, err := r.currentSubject(c)
	if err != nil {
		return err
	}

	// permissions on role binding actions, similar to roles v1, are granted on the resources
	if err := r.checkActionWithResponse(ctx, actor, string(iapl.RoleBindingActionGet), resource); err != nil {
		return err
	}

	rb, err := r.engine.GetRoleBinding(ctx, rbRes)
	if err != nil {
		return r.errorResponse("error getting role-binding", err)
	}

	return c.JSON(
		http.StatusOK,
		roleBindingResponse{
			ID:       rb.ID,
			Subjects: resourceToSubject(rb.Subjects),
			Role: roleBindingResponseRole{
				ID:   rb.Role.ID,
				Name: rb.Role.Name,
			},
		},
	)
}

func (r *Router) roleBindingUpdate(c echo.Context) error {
	resID := c.Param("id")
	rbID := c.Param("rb_id")

	ctx, span := tracer.Start(
		c.Request().Context(), "api.roleBindingUpdate",
		trace.WithAttributes(attribute.String("id", resID)),
		trace.WithAttributes(attribute.String("rolebinding_id", rbID)),
	)
	defer span.End()

	// resource
	resourceID, err := gidx.Parse(resID)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "error parsing resource ID").SetInternal(err)
	}

	resource, err := r.engine.NewResourceFromID(resourceID)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "error creating resource").SetInternal(err)
	}

	// role-binding
	rolebindingID, err := gidx.Parse(rbID)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "error parsing resource ID").SetInternal(err)
	}

	rbRes, err := r.engine.NewResourceFromID(rolebindingID)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "error creating resource").SetInternal(err)
	}

	actor, err := r.currentSubject(c)
	if err != nil {
		return err
	}

	// permissions on role binding actions, similar to roles v1, are granted on the resources
	if err := r.checkActionWithResponse(ctx, actor, string(iapl.RoleBindingActionUpdate), resource); err != nil {
		return err
	}

	body := &rolebindingUpdateRequest{}

	err = c.Bind(body)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "error parsing request body").SetInternal(err)
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

	rb, err := r.engine.UpdateRoleBinding(ctx, rbRes, subjects)
	if err != nil {
		return r.errorResponse("error updating role-binding", err)
	}

	return c.JSON(
		http.StatusOK,
		roleBindingResponse{
			ID:       rb.ID,
			Subjects: resourceToSubject(rb.Subjects),
			Role: roleBindingResponseRole{
				ID:   rb.Role.ID,
				Name: rb.Role.Name,
			},
		},
	)
}
