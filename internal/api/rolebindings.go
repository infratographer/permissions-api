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
	actionRoleBindingGet    = "rolebinding_get"
	actionRoleBindingList   = "rolebinding_list"
	actionRoleBindingUpdate = "rolebinding_update"
	actionRoleBindingDelete = "rolebinding_delete"
)

func resourceToSubject(resources []types.Resource) []roleBindingSubject {
	resp := make([]roleBindingSubject, len(resources))
	for i, res := range resources {
		resp[i] = roleBindingSubject{
			ID:   res.ID,
			Type: res.Type,
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

	var body createRoleBindingRequest

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

	rolebinding, err := r.engine.CreateRoleBinding(ctx, roleResource, resource, body.Subjects)
	if err != nil {
		return r.errorResponse("error creating role-binding", err)
	}

	resp := roleBindingResponse{
		ID:       rolebinding.ID,
		Subjects: resourceToSubject(rolebinding.Subjects),
		Role: roleBindingResponseRole{
			ID:   rolebinding.Role.ID,
			Name: rolebinding.Role.Name,
		},
	}

	return c.JSON(http.StatusCreated, resp)
}

func (r *Router) roleBindingGet(c echo.Context) error {
	resourceIDStr := c.Param("id")
	rbIDStr := c.Param("rb_id")

	ctx, span := tracer.Start(
		c.Request().Context(), "api.roleBindingGet",
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

	rbID, err := gidx.Parse(rbIDStr)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "error parsing role-binding ID").SetInternal(err)
	}

	roleBindingResource, err := r.engine.NewResourceFromID(rbID)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "error getting role-binding resource").SetInternal(err)
	}

	subjectResource, err := r.currentSubject(c)
	if err != nil {
		return err
	}

	if err := r.checkActionWithResponse(ctx, subjectResource, actionRoleCreate, resource); err != nil {
		return err
	}

	rb, err := r.engine.GetRoleBinding(ctx, resource, roleBindingResource)
	if err != nil {
		return r.errorResponse("error getting role-binding", err)
	}

	resp := roleBindingResponse{
		ID:       rb.ID,
		Subjects: resourceToSubject(rb.Subjects),
		Role: roleBindingResponseRole{
			ID:   rb.Role.ID,
			Name: rb.Role.Name,
		},
	}

	return c.JSON(http.StatusOK, resp)
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

	if err := r.checkActionWithResponse(ctx, subjectResource, actionRoleCreate, resource); err != nil {
		return err
	}

	rbs, err := r.engine.ListRoleBindings(ctx, resource)
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
