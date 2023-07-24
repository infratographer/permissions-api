package permissions

import (
	"context"

	"github.com/labstack/echo/v4"
	"go.infratographer.com/x/gidx"
)

var (
	// CheckerCtxKey is the context key used to set the checker handling function
	CheckerCtxKey = checkerCtxKey{}

	// DefaultAllowChecker defaults to allow when checker is disabled or skipped
	DefaultAllowChecker Checker = func(_ context.Context, _ ...AccessRequest) error {
		return nil
	}

	// DefaultDenyChecker defaults to denied when checker is disabled or skipped
	DefaultDenyChecker Checker = func(_ context.Context, _ ...AccessRequest) error {
		return ErrPermissionDenied
	}
)

// Checker defines the checker function definition
type Checker func(ctx context.Context, requests ...AccessRequest) error

// AccessRequest defines the required fields to check permissions access.
type AccessRequest struct {
	ResourceID gidx.PrefixedID `json:"resource_id"`
	Action     string          `json:"action"`
}

type checkerCtxKey struct{}

func setCheckerContext(c echo.Context, checker Checker) {
	if checker == nil {
		checker = DefaultDenyChecker
	}

	req := c.Request().WithContext(
		context.WithValue(
			c.Request().Context(),
			CheckerCtxKey,
			checker,
		),
	)

	c.SetRequest(req)
}

// CheckAccess runs the checker function to check if the provided resource and action are supported.
func CheckAccess(ctx context.Context, resource gidx.PrefixedID, action string) error {
	checker, ok := ctx.Value(CheckerCtxKey).(Checker)
	if !ok {
		return ErrCheckerNotFound
	}

	request := AccessRequest{
		ResourceID: resource,
		Action:     action,
	}

	return checker(ctx, request)
}

// CheckAll runs the checker function to check if all the provided resources and actions are permitted.
func CheckAll(ctx context.Context, requests ...AccessRequest) error {
	checker, ok := ctx.Value(CheckerCtxKey).(Checker)
	if !ok {
		return ErrCheckerNotFound
	}

	return checker(ctx, requests...)
}
