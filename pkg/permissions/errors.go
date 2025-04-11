package permissions

import (
	"errors"
	"fmt"

	"github.com/labstack/echo/v4"
)

var (
	// Error is the root error for all permissions related errors.
	Error = errors.New("permissions error")

	// AuthError is the root error all auth related errors stem from.
	AuthError = fmt.Errorf("%w: auth", Error) //nolint:revive,staticcheck // not returned directly, but used as a root error.

	// ErrNoAuthToken is the error returned when there is no auth token provided for the API request
	ErrNoAuthToken = echo.ErrBadRequest.WithInternal(fmt.Errorf("%w: no auth token provided for client", AuthError))

	// ErrInvalidAuthToken is the error returned when the auth token is not the expected value
	ErrInvalidAuthToken = echo.ErrBadRequest.WithInternal(fmt.Errorf("%w: invalid auth token", AuthError))

	// ErrPermissionDenied is the error returned when permission is denied to a call
	ErrPermissionDenied = echo.ErrUnauthorized.WithInternal(fmt.Errorf("%w: subject doesn't have access", AuthError))

	// ErrBadResponse is the error returned when we receive a bad response from the server
	ErrBadResponse = fmt.Errorf("%w: bad response from server", Error)

	// ErrCheckerNotFound is the error returned when CheckAccess does not find the appropriate checker context
	ErrCheckerNotFound = fmt.Errorf("%w: no checker found in context", Error)

	// ErrPermissionsMiddlewareMissing is returned when a permissions method has been called but the middleware is missing.
	ErrPermissionsMiddlewareMissing = fmt.Errorf("%w: permissions middleware missing", Error)
)
