package api

import (
	"errors"
	"net/http"

	"go.infratographer.com/x/gidx"

	"go.infratographer.com/permissions-api/internal/iapl"
	"go.infratographer.com/permissions-api/internal/query"
	"go.infratographer.com/permissions-api/internal/types"

	"github.com/labstack/echo/v4"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

func (r *Router) assignmentCreate(c echo.Context) error {
	roleIDStr := c.Param("role_id")

	roleID, err := gidx.Parse(roleIDStr)
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

	assigneeID, err := gidx.Parse(reqBody.SubjectID)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "error parsing subject ID").SetInternal(err)
	}

	assigneeResource, err := r.engine.NewResourceFromID(assigneeID)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "error assigning subject").SetInternal(err)
	}

	subjectResource, err := r.currentSubject(c)
	if err != nil {
		return err
	}

	roleResource, err := r.engine.NewResourceFromID(roleID)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "error getting resource").SetInternal(err)
	}

	resource, err := r.engine.GetRoleResource(ctx, roleResource)

	switch {
	case err == nil:
	case errors.Is(err, query.ErrRoleNotFound):
		return echo.NewHTTPError(http.StatusNotFound, "role not found").SetInternal(err)
	default:
		return echo.NewHTTPError(http.StatusInternalServerError, "error getting role").SetInternal(err)
	}

	if err := r.checkActionWithResponse(ctx, subjectResource, string(iapl.RoleActionUpdate), resource); err != nil {
		return err
	}

	role := types.Role{
		ID: roleID,
	}

	if err = r.engine.AssignSubjectRole(ctx, assigneeResource, role); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "error creating resource").SetInternal(err)
	}

	resp := createAssignmentResponse{
		Success: true,
	}

	return c.JSON(http.StatusCreated, resp)
}

func (r *Router) assignmentsList(c echo.Context) error {
	roleIDStr := c.Param("role_id")

	roleID, err := gidx.Parse(roleIDStr)
	if err != nil {
		return echo.ErrNotFound
	}

	ctx, span := tracer.Start(c.Request().Context(), "api.assignmentCreate", trace.WithAttributes(attribute.String("role_id", roleIDStr)))
	defer span.End()

	subjectResource, err := r.currentSubject(c)
	if err != nil {
		return err
	}

	roleResource, err := r.engine.NewResourceFromID(roleID)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "error getting resource").SetInternal(err)
	}

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

	role := types.Role{
		ID: roleID,
	}

	assignments, err := r.engine.ListAssignments(ctx, role)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "error listing assignments").SetInternal(err)
	}

	items := make([]assignmentItem, len(assignments))

	for i, res := range assignments {
		item := assignmentItem{
			SubjectID: res.ID.String(),
		}

		items[i] = item
	}

	out := listAssignmentsResponse{
		Data: items,
	}

	return c.JSON(http.StatusOK, out)
}

func (r *Router) assignmentDelete(c echo.Context) error {
	roleIDStr := c.Param("role_id")

	roleID, err := gidx.Parse(roleIDStr)
	if err != nil {
		return echo.ErrNotFound
	}

	ctx, span := tracer.Start(c.Request().Context(), "api.assignmentDelete", trace.WithAttributes(attribute.String("role_id", roleIDStr)))
	defer span.End()

	var reqBody deleteAssignmentRequest

	err = c.Bind(&reqBody)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "error parsing request body").SetInternal(err)
	}

	assigneeID, err := gidx.Parse(reqBody.SubjectID)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "error parsing subject ID").SetInternal(err)
	}

	assigneeResource, err := r.engine.NewResourceFromID(assigneeID)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "error parsing resource type from subject").SetInternal(err)
	}

	subjectResource, err := r.currentSubject(c)
	if err != nil {
		return err
	}

	roleResource, err := r.engine.NewResourceFromID(roleID)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "error getting resource").SetInternal(err)
	}

	resource, err := r.engine.GetRoleResource(ctx, roleResource)

	switch {
	case err == nil:
	case errors.Is(err, query.ErrRoleNotFound):
		return echo.NewHTTPError(http.StatusNotFound, "role not found").SetInternal(err)
	default:
		return echo.NewHTTPError(http.StatusInternalServerError, "error getting role").SetInternal(err)
	}

	if err := r.checkActionWithResponse(ctx, subjectResource, string(iapl.RoleActionUpdate), resource); err != nil {
		return err
	}

	role := types.Role{
		ID: roleID,
	}

	if err = r.engine.UnassignSubjectRole(ctx, assigneeResource, role); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "error deleting assignment").SetInternal(err)
	}

	resp := deleteAssignmentResponse{
		Success: true,
	}

	return c.JSON(http.StatusOK, resp)
}
