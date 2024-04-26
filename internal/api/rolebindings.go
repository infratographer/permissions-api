package api

import (
	"fmt"
	"net/http"
	"time"

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
			ID:   subj.SubjectResource.ID,
			Type: subj.SubjectResource.Type,
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
		return r.errorResponse("error parsing resource ID", fmt.Errorf("%w: %s", ErrInvalidID, err.Error()))
	}

	var body roleBindingRequest

	err = c.Bind(&body)
	if err != nil {
		return r.errorResponse(err.Error(), ErrParsingRequestBody)
	}

	resource, err := r.engine.NewResourceFromID(resourceID)
	if err != nil {
		return r.errorResponse("error creating resource", err)
	}

	actor, err := r.currentSubject(c)
	if err != nil {
		return err
	}

	// permissions on role binding actions, similar to roles v1, are granted on the resources
	if err := r.checkActionWithResponse(ctx, actor, string(iapl.RoleBindingActionCreate), resource); err != nil {
		return err
	}

	roleID, err := gidx.Parse(body.RoleID)
	if err != nil {
		return r.errorResponse("error parsing role ID", fmt.Errorf("%w: %s", ErrInvalidID, err.Error()))
	}

	roleResource, err := r.engine.NewResourceFromID(roleID)
	if err != nil {
		return r.errorResponse("error creating role resource", err)
	}

	subjects := make([]types.RoleBindingSubject, len(body.Subjects))

	for i, s := range body.Subjects {
		subj, err := r.engine.NewResourceFromID(s.ID)
		if err != nil {
			return r.errorResponse("error creating subject resource", err)
		}

		subjects[i] = types.RoleBindingSubject{
			SubjectResource: subj,
			Condition:       nil,
		}
	}

	rb, err := r.engine.CreateRoleBinding(ctx, actor, resource, roleResource, subjects)
	if err != nil {
		return r.errorResponse("error creating role-binding", err)
	}

	return c.JSON(
		http.StatusOK,
		roleBindingResponse{
			ID:         rb.ID,
			ResourceID: rb.ResourceID,
			Subjects:   resourceToSubject(rb.Subjects),
			Role: roleBindingResponseRole{
				ID:   rb.Role.ID,
				Name: rb.Role.Name,
			},

			CreatedBy: rb.CreatedBy,
			UpdatedBy: rb.UpdatedBy,
			CreatedAt: rb.CreatedAt.Format(time.RFC3339),
			UpdatedAt: rb.UpdatedAt.Format(time.RFC3339),
		},
	)
}

func (r *Router) roleBindingsList(c echo.Context) error {
	resourceIDStr := c.Param("id")

	ctx, span := tracer.Start(
		c.Request().Context(), "api.roleBindingList",
		trace.WithAttributes(attribute.String("id", resourceIDStr)),
	)
	defer span.End()

	resourceID, err := gidx.Parse(resourceIDStr)
	if err != nil {
		return r.errorResponse("error parsing resource ID", fmt.Errorf("%w: %s", ErrInvalidID, err.Error()))
	}

	resource, err := r.engine.NewResourceFromID(resourceID)
	if err != nil {
		return r.errorResponse("error creating resource", err)
	}

	subjectResource, err := r.currentSubject(c)
	if err != nil {
		return err
	}

	if err := r.checkActionWithResponse(ctx, subjectResource, string(iapl.RoleBindingActionList), resource); err != nil {
		return err
	}

	rbs, err := r.engine.ListRoleBindings(ctx, resource, nil)
	if err != nil {
		return r.errorResponse("error listing role-binding", err)
	}

	resp := listRoleBindingsResponse{
		Data: make([]roleBindingResponse, len(rbs)),
	}

	for i, rb := range rbs {
		resp.Data[i] = roleBindingResponse{
			ID:         rb.ID,
			ResourceID: rb.ResourceID,
			Subjects:   resourceToSubject(rb.Subjects),
			Role: roleBindingResponseRole{
				ID:   rb.Role.ID,
				Name: rb.Role.Name,
			},

			CreatedBy: rb.CreatedBy,
			UpdatedBy: rb.UpdatedBy,
			CreatedAt: rb.CreatedAt.Format(time.RFC3339),
			UpdatedAt: rb.UpdatedAt.Format(time.RFC3339),
		}
	}

	return c.JSON(http.StatusOK, resp)
}

func (r *Router) roleBindingsDelete(c echo.Context) error {
	rbID := c.Param("rb_id")

	ctx, span := tracer.Start(
		c.Request().Context(), "api.roleBindingDelete",
		trace.WithAttributes(attribute.String("id", rbID)),
	)
	defer span.End()

	// role-binding
	rolebindingID, err := gidx.Parse(rbID)
	if err != nil {
		return r.errorResponse("error parsing resource ID", fmt.Errorf("%w: %s", ErrInvalidID, err.Error()))
	}

	rbRes, err := r.engine.NewResourceFromID(rolebindingID)
	if err != nil {
		return r.errorResponse("error creating resource", err)
	}

	actor, err := r.currentSubject(c)
	if err != nil {
		return err
	}

	// resource
	resource, err := r.engine.GetRoleBindingResource(ctx, rbRes)
	if err != nil {
		return r.errorResponse("error getting role-binding owner resource", err)
	}

	// permissions on role binding actions, similar to roles v1, are granted on the resources
	if err := r.checkActionWithResponse(ctx, actor, string(iapl.RoleBindingActionDelete), resource); err != nil {
		return err
	}

	if err := r.engine.DeleteRoleBinding(ctx, rbRes); err != nil {
		return r.errorResponse("error updating role-binding", err)
	}

	resp := deleteRoleBindingResponse{Success: true}

	return c.JSON(http.StatusOK, resp)
}

func (r *Router) roleBindingGet(c echo.Context) error {
	rbID := c.Param("rb_id")

	ctx, span := tracer.Start(
		c.Request().Context(), "api.roleBindingGet",
		trace.WithAttributes(attribute.String("id", rbID)),
	)
	defer span.End()

	// role-binding
	rolebindingID, err := gidx.Parse(rbID)
	if err != nil {
		return r.errorResponse("error parsing resource ID", fmt.Errorf("%w: %s", ErrInvalidID, err.Error()))
	}

	rbRes, err := r.engine.NewResourceFromID(rolebindingID)
	if err != nil {
		return r.errorResponse("error creating resource", err)
	}

	actor, err := r.currentSubject(c)
	if err != nil {
		return err
	}

	rb, err := r.engine.GetRoleBinding(ctx, rbRes)
	if err != nil {
		return r.errorResponse("error getting role-binding", err)
	}

	// permissions on role binding actions, similar to roles v1, are granted on the resources
	// since the rolebinding is returning the resource ID that it belongs to, we
	// will use this resource ID to check the permissions
	resource, err := r.engine.NewResourceFromID(rb.ResourceID)
	if err != nil {
		return r.errorResponse("error creating resource", err)
	}

	if err := r.checkActionWithResponse(ctx, actor, string(iapl.RoleBindingActionGet), resource); err != nil {
		return err
	}

	return c.JSON(
		http.StatusOK,
		roleBindingResponse{
			ID:         rb.ID,
			ResourceID: rb.ResourceID,
			Subjects:   resourceToSubject(rb.Subjects),
			Role: roleBindingResponseRole{
				ID:   rb.Role.ID,
				Name: rb.Role.Name,
			},

			CreatedBy: rb.CreatedBy,
			UpdatedBy: rb.UpdatedBy,
			CreatedAt: rb.CreatedAt.Format(time.RFC3339),
			UpdatedAt: rb.UpdatedAt.Format(time.RFC3339),
		},
	)
}

func (r *Router) roleBindingUpdate(c echo.Context) error {
	rbID := c.Param("rb_id")

	ctx, span := tracer.Start(
		c.Request().Context(), "api.roleBindingUpdate",
		trace.WithAttributes(attribute.String("rolebinding_id", rbID)),
	)
	defer span.End()

	// resource

	// role-binding
	rolebindingID, err := gidx.Parse(rbID)
	if err != nil {
		return r.errorResponse("error parsing resource ID", fmt.Errorf("%w: %s", ErrInvalidID, err.Error()))
	}

	rbRes, err := r.engine.NewResourceFromID(rolebindingID)
	if err != nil {
		return r.errorResponse("error creating resource", err)
	}

	actor, err := r.currentSubject(c)
	if err != nil {
		return err
	}

	// resource
	resource, err := r.engine.GetRoleBindingResource(ctx, rbRes)
	if err != nil {
		return r.errorResponse("error getting role-binding owner resource", err)
	}

	// permissions on role binding actions, similar to roles v1, are granted on the resources
	if err := r.checkActionWithResponse(ctx, actor, string(iapl.RoleBindingActionUpdate), resource); err != nil {
		return err
	}

	body := &rolebindingUpdateRequest{}

	err = c.Bind(body)
	if err != nil {
		return r.errorResponse(err.Error(), ErrParsingRequestBody)
	}

	subjects := make([]types.RoleBindingSubject, len(body.Subjects))

	for i, s := range body.Subjects {
		subj, err := r.engine.NewResourceFromID(s.ID)
		if err != nil {
			return r.errorResponse("error creating subject resource", err)
		}

		subjects[i] = types.RoleBindingSubject{
			SubjectResource: subj,
			Condition:       nil,
		}
	}

	rb, err := r.engine.UpdateRoleBinding(ctx, actor, rbRes, subjects)
	if err != nil {
		return r.errorResponse("error updating role-binding", err)
	}

	return c.JSON(
		http.StatusOK,
		roleBindingResponse{
			ID:         rb.ID,
			ResourceID: rb.ResourceID,
			Subjects:   resourceToSubject(rb.Subjects),

			Role: roleBindingResponseRole{
				ID:   rb.Role.ID,
				Name: rb.Role.Name,
			},

			CreatedBy: rb.CreatedBy,
			UpdatedBy: rb.UpdatedBy,
			CreatedAt: rb.CreatedAt.Format(time.RFC3339),
			UpdatedAt: rb.UpdatedAt.Format(time.RFC3339),
		},
	)
}
