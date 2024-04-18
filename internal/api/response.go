package api

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/labstack/echo/v4"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"go.infratographer.com/permissions-api/internal/query"
	"go.infratographer.com/permissions-api/internal/storage"
)

// ErrorResponse represents the data that the server will return on any given call
type ErrorResponse struct {
	Message string `json:"message,omitempty"`
	Error   string `json:"error,omitempty"`
}

var (
	// ErrResourceNotFound is returned when the requested resource isn't found
	ErrResourceNotFound = errors.New("resource not found")

	// ErrSearchNotFound is returned when the requested search term isn't found
	ErrSearchNotFound = errors.New("search term not found")

	// ErrResourceAlreadyExists is returned when the resource already exists
	ErrResourceAlreadyExists = errors.New("resource already exists")
)

func (r *Router) errorResponse(basemsg string, err error) *echo.HTTPError {
	msg := fmt.Sprintf("%s: %s", basemsg, err.Error())
	httpstatus := http.StatusInternalServerError

	switch {
	case
		errors.Is(err, query.ErrInvalidType),
		errors.Is(err, query.ErrInvalidArgument),
		errors.Is(err, query.ErrInvalidAction),
		errors.Is(err, query.ErrInvalidNamespace),
		errors.Is(err, ErrInvalidID),
		status.Code(err) == codes.InvalidArgument,
		status.Code(err) == codes.FailedPrecondition:
		httpstatus = http.StatusBadRequest
	case
		errors.Is(err, storage.ErrNoRoleFound),
		errors.Is(err, query.ErrRoleNotFound),
		errors.Is(err, query.ErrRoleBindingNotFound):
		httpstatus = http.StatusNotFound
	case
		errors.Is(err, storage.ErrRoleAlreadyExists),
		errors.Is(err, storage.ErrRoleNameTaken):
		httpstatus = http.StatusConflict
	default:
		msg = basemsg
	}

	return echo.NewHTTPError(httpstatus, msg).SetInternal(err)
}
