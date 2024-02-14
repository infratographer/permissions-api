package api

import (
	"net/http"

	"github.com/labstack/echo/v4"
	"go.infratographer.com/x/gidx"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

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

	if err := r.checkActionWithResponse(ctx, subjectResource, actionRoleCreate, resource); err != nil {
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
		Subjects: body.Subjects,
		Role: roleBindingResponseRole{
			ID:   rolebinding.Role.ID,
			Name: rolebinding.Role.Name,
		},
	}

	return c.JSON(http.StatusCreated, resp)
}
