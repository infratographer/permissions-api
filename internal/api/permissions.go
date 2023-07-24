package api

import (
	"errors"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/labstack/echo/v4"
	"go.infratographer.com/permissions-api/internal/query"
	"go.infratographer.com/permissions-api/internal/types"
	"go.infratographer.com/x/gidx"
	"go.uber.org/multierr"
)

const maxCheckDuration = 5 * time.Second

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

type checkStatus struct {
	Resource types.Resource
	Action   string
	Error    error
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

	results := make([]*checkStatus, len(reqBody.Actions))

	for i, check := range reqBody.Actions {
		if check.Action == "" {
			errs = append(errs, fmt.Errorf("check %d: no action defined", i))

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

		results[i] = &checkStatus{
			Resource: resource,
			Action:   check.Action,
		}
	}

	if len(errs) != 0 {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid check request").SetInternal(multierr.Combine(errs...))
	}

	checkCh := make(chan *checkStatus)

	wg := new(sync.WaitGroup)

	for i := 0; i < r.concurrentChecks; i++ {
		wg.Add(1)

		go func() {
			defer wg.Done()

			for check := range checkCh {
				// Check the permissions
				err := r.engine.SubjectHasPermission(ctx, subjectResource, check.Action, check.Resource)
				if err != nil {
					check.Error = err
				}
			}
		}()
	}

	wg.Add(1)

	go func() {
		defer wg.Done()

		for _, check := range results {
			checkCh <- check
		}

		close(checkCh)
	}()

	doneCh := make(chan struct{})

	go func() {
		defer close(doneCh)

		wg.Wait()
	}()

	select {
	case <-doneCh:
	case <-ctx.Done():
		return echo.NewHTTPError(http.StatusInternalServerError, "request cancelled").WithInternal(ctx.Err())
	case <-time.After(maxCheckDuration):
		return echo.NewHTTPError(http.StatusInternalServerError, "checks didn't complete in time")
	}

	var (
		unauthorizedErrors []error
		internalErrors     []error
		allErrors          []error
	)

	for i, check := range results {
		if check.Error != nil {
			if errors.Is(check.Error, query.ErrActionNotAssigned) {
				err := fmt.Errorf("subject '%s' does not have permission to perform action '%s' on resource '%s'",
					subject, check.Action, check.Resource.ID.String())

				unauthorizedErrors = append(unauthorizedErrors, err)
				allErrors = append(allErrors, err)
			} else {
				err := fmt.Errorf("check %d: %w", i, check.Error)

				internalErrors = append(internalErrors, err)
				allErrors = append(allErrors, err)
			}
		}
	}

	if len(internalErrors) != 0 {
		return echo.NewHTTPError(http.StatusInternalServerError, "an error occurred checking permissions").SetInternal(multierr.Combine(allErrors...))
	}

	if len(unauthorizedErrors) != 0 {
		msg := fmt.Sprintf("subject '%s' does not have permission to the requests resource actions", subject)

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
