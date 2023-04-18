package api

import (
	"net/http"

	"go.infratographer.com/permissions-api/internal/types"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"go.infratographer.com/x/urnx"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

func (r *Router) assignmentCreate(c echo.Context) error {
	roleIDStr := c.Param("role_id")

	roleID, err := uuid.Parse(roleIDStr)
	if err != nil {
		return echo.ErrNotFound
	}

	ctx, span := tracer.Start(c.Request().Context(), "api.assignmentCreate", trace.WithAttributes(attribute.String("role_id", roleIDStr)))
	defer span.End()

	var reqBody createAssignmentRequest

	err = c.Bind(&reqBody)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "error parsing request body").SetInternal(err)
	}

	subjURN, err := urnx.Parse(reqBody.SubjectURN)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "error parsing subject URN").SetInternal(err)
	}

	subjResource, err := r.engine.NewResourceFromURN(subjURN)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "error creating resource").SetInternal(err)
	}

	role := types.Role{
		ID: roleID,
	}

	_, err = r.engine.AssignSubjectRole(ctx, subjResource, role)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "error creating resource").SetInternal(err)
	}

	resp := createAssignmentResponse{
		Success: true,
	}

	return c.JSON(http.StatusCreated, resp)
}

func (r *Router) assignmentsList(c echo.Context) error {
	roleIDStr := c.Param("role_id")

	roleID, err := uuid.Parse(roleIDStr)
	if err != nil {
		return echo.ErrNotFound
	}

	ctx, span := tracer.Start(c.Request().Context(), "api.assignmentCreate", trace.WithAttributes(attribute.String("role_id", roleIDStr)))
	defer span.End()

	role := types.Role{
		ID: roleID,
	}

	assignments, err := r.engine.ListAssignments(ctx, role, "")
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "error listing assignments").SetInternal(err)
	}

	items := make([]assignmentItem, len(assignments))

	for i, res := range assignments {
		subjURN, err := r.engine.NewURNFromResource(res)
		if err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, "error listing assignments").SetInternal(err)
		}

		item := assignmentItem{
			SubjectURN: subjURN.String(),
		}

		items[i] = item
	}

	out := listAssignmentsResponse{
		Data: items,
	}

	return c.JSON(http.StatusOK, out)
}
