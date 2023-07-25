package api

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
	"go.infratographer.com/permissions-api/internal/query"
	"go.infratographer.com/permissions-api/internal/types"
	"go.infratographer.com/x/gidx"
	"go.uber.org/multierr"
)

const (
	defaultMaxCheckConcurrency = 5

	maxCheckDuration = 5 * time.Second
)

var (
	// ErrNoActionDefined is the error returned when an access request is has no action defined
	ErrNoActionDefined = errors.New("no action defined")

	// ErrAccessDenied is returned when access is denied
	ErrAccessDenied = errors.New("access denied")
)

// checkAction will check if a subject is allowed to perform an action on a resource.
// This is the permissions check endpoint.
// It will return a 200 if the subject is allowed to perform the action on the resource.
// It will return a 403 if the subject is not allowed to perform the action on the resource.
//
// Note that this expects a JWT token to be present in the request. This token must
// contain the subject of the request in the "sub" claim.
//
// The following query parameters are required:
// - resource: the resource ID to check
// - action: the action to check
func (r *Router) checkAction(c echo.Context) error {
	ctx, span := tracer.Start(c.Request().Context(), "api.checkAction")
	defer span.End()

	action, hasQuery := getParam(c, "action")
	if !hasQuery {
		return echo.NewHTTPError(http.StatusBadRequest, "missing action query parameter")
	}

	// Optional query parameters
	resourceIDStr, hasResourceParam := getParam(c, "resource")
	if !hasResourceParam {
		return echo.NewHTTPError(http.StatusBadRequest, "missing resource query parameter")
	}

	// Query parameter validation
	resourceID, err := gidx.Parse(resourceIDStr)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "error processing resource ID").SetInternal(err)
	}

	resource, err := r.engine.NewResourceFromID(resourceID)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "error processing tenant resource ID").SetInternal(err)
	}

	// Subject validation
	subject, err := currentSubject(c)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "failed to get the subject").SetInternal(err)
	}

	subjectResource, err := r.engine.NewResourceFromID(subject)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "error processing subject ID").SetInternal(err)
	}

	// Check the permissions
	err = r.engine.SubjectHasPermission(ctx, subjectResource, action, resource)
	if err != nil && errors.Is(err, query.ErrActionNotAssigned) {
		msg := fmt.Sprintf("subject '%s' does not have permission to perform action '%s' on resource '%s'",
			subject, action, resourceIDStr)

		return echo.NewHTTPError(http.StatusForbidden, msg).SetInternal(err)
	} else if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "an error occurred checking permissions").SetInternal(err)
	}

	return c.JSON(http.StatusOK, echo.Map{})
}

type checkPermissionsRequest struct {
	Actions []checkAction `json:"actions"`
}

type checkAction struct {
	ResourceID string `json:"resource_id"`
	Action     string `json:"action"`
}

type checkRequest struct {
	Index    int
	Resource types.Resource
	Action   string
}

type checkResult struct {
	Request checkRequest
	Error   error
}

// checkAllActions will check if a subject is allowed to perform an action on a list of resources.
// This is the permissions check endpoint.
// It will return a 200 if the subject is allowed to perform all requested resource actions.
// It will return a 400 if the request is invalid.
// It will return a 403 if the subject is not allowed to perform all requested resource actions.
//
// Note that this expects a JWT token to be present in the request. This token must
// contain the subject of the request in the "sub" claim.
func (r *Router) checkAllActions(c echo.Context) error {
	ctx, span := tracer.Start(c.Request().Context(), "api.checkAllActions")
	defer span.End()

	// Subject validation
	subject, err := currentSubject(c)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "failed to get the subject").SetInternal(err)
	}

	subjectResource, err := r.engine.NewResourceFromID(subject)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "error processing subject ID").SetInternal(err)
	}

	var reqBody checkPermissionsRequest

	if err := c.Bind(&reqBody); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "error parsing request body").SetInternal(err)
	}

	var errs []error

	requestsCh := make(chan checkRequest, len(reqBody.Actions))

	for i, check := range reqBody.Actions {
		if check.Action == "" {
			errs = append(errs, fmt.Errorf("check %d: %w", i, ErrNoActionDefined))

			continue
		}

		resourceID, err := gidx.Parse(check.ResourceID)
		if err != nil {
			errs = append(errs, fmt.Errorf("check %d: %w: error parsing resource id: %s", i, err, check.ResourceID))

			continue
		}

		resource, err := r.engine.NewResourceFromID(resourceID)
		if err != nil {
			errs = append(errs, fmt.Errorf("check %d: %w: error creating resource from id: %s", i, err, resourceID.String()))

			continue
		}

		requestsCh <- checkRequest{
			Index:    i,
			Resource: resource,
			Action:   check.Action,
		}
	}

	close(requestsCh)

	if len(errs) != 0 {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid check request").SetInternal(multierr.Combine(errs...))
	}

	resultsCh := make(chan checkResult, len(reqBody.Actions))

	ctx, cancel := context.WithTimeout(ctx, maxCheckDuration)

	defer cancel()

	for i := 0; i < r.concurrentChecks; i++ {
		go func() {
			for {
				var result checkResult

				select {
				case check, ok := <-requestsCh:
					// if channel is closed, quit the go routine.
					if !ok {
						return
					}

					result.Request = check

					// Check the permissions
					err := r.engine.SubjectHasPermission(ctx, subjectResource, check.Action, check.Resource)
					if err != nil {
						result.Error = err
					}
				case <-ctx.Done():
					result.Error = ctx.Err()
				}

				resultsCh <- result
			}
		}()
	}

	var (
		unauthorizedErrors int
		internalErrors     int
		allErrors          []error
	)

	for i := 0; i < len(reqBody.Actions); i++ {
		select {
		case result := <-resultsCh:
			if result.Error != nil {
				if errors.Is(result.Error, query.ErrActionNotAssigned) {
					err := fmt.Errorf("%w: subject '%s' does not have permission to perform action '%s' on resource '%s'",
						ErrAccessDenied, subject, result.Request.Action, result.Request.Resource.ID.String())

					unauthorizedErrors++

					allErrors = append(allErrors, err)
				} else {
					err := fmt.Errorf("check %d: %w", result.Request.Index, result.Error)

					internalErrors++

					allErrors = append(allErrors, err)
				}
			}
		case <-ctx.Done():
			if err := ctx.Err(); err != nil {
				internalErrors++

				allErrors = append(allErrors, ctx.Err())
			}
		}
	}

	if internalErrors != 0 {
		return echo.NewHTTPError(http.StatusInternalServerError, "an error occurred checking permissions").SetInternal(multierr.Combine(allErrors...))
	}

	if unauthorizedErrors != 0 {
		msg := fmt.Sprintf("subject '%s' does not have permission to the requested resource actions", subject)

		return echo.NewHTTPError(http.StatusForbidden, msg).SetInternal(multierr.Combine(allErrors...))
	}

	return nil
}

func getParam(c echo.Context, name string) (string, bool) {
	values, ok := c.QueryParams()[name]
	if !ok {
		return "", ok
	}

	if len(values) == 0 {
		return "", true
	}

	return values[0], true
}
