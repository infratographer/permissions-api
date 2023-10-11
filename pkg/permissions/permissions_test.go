package permissions_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/require"
	"go.infratographer.com/x/echojwtx"
	"go.infratographer.com/x/gidx"

	"go.infratographer.com/permissions-api/pkg/permissions"
)

func TestPermissions(t *testing.T) {
	allowedID := gidx.MustNewID("testgid")
	deniedID := gidx.MustNewID("testgid")

	actions := map[string]bool{
		"resource_create": true,
		"resource_update": true,
		"resource_delete": false,
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authToken := r.Header.Get("Authorization")
		if authToken != "Bearer good-token" {
			w.WriteHeader(http.StatusUnauthorized)

			return
		}

		var reqBody struct {
			Actions []struct {
				ResourceID string `json:"resource_id"`
				Action     string `json:"action"`
			} `json:"actions"`
		}

		if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
			w.WriteHeader(http.StatusInternalServerError)

			return
		}

		for _, request := range reqBody.Actions {
			resource, err := gidx.Parse(request.ResourceID)
			if err != nil {
				w.WriteHeader(http.StatusBadRequest)

				return
			}

			action := request.Action

			if resource != allowedID && resource != deniedID {
				w.WriteHeader(http.StatusInternalServerError)

				return
			}

			if resource != allowedID || !actions[action] {
				w.WriteHeader(http.StatusForbidden)

				return
			}
		}
	}))

	testCases := []struct {
		name                  string
		config                permissions.Config
		options               []permissions.Option
		authHeader            string
		resource              gidx.PrefixedID
		action                string
		expectMiddlewareError error
		expectCheckError      error
	}{
		{
			"no config default deny",
			permissions.Config{},
			nil,
			"",
			"somersc-abc123",
			"some-action",
			nil,
			permissions.ErrPermissionDenied,
		},
		{
			"no config with default deny",
			permissions.Config{},
			[]permissions.Option{permissions.WithDefaultChecker(permissions.DefaultDenyChecker)},
			"",
			"somersc-abc123",
			"some-action",
			nil,
			permissions.ErrPermissionDenied,
		},
		{
			"no config with default allow",
			permissions.Config{},
			[]permissions.Option{permissions.WithDefaultChecker(permissions.DefaultAllowChecker)},
			"",
			"somersc-abc123",
			"some-action",
			nil,
			nil,
		},
		{
			"check allowed",
			permissions.Config{
				URL: srv.URL,
			},
			nil,
			"Bearer good-token",
			allowedID,
			"resource_create",
			nil,
			nil,
		},
		{
			"check denied",
			permissions.Config{
				URL: srv.URL,
			},
			nil,
			"Bearer good-token",
			allowedID,
			"resource_delete",
			nil,
			permissions.ErrPermissionDenied,
		},
		{
			"check denied",
			permissions.Config{
				URL: srv.URL,
			},
			nil,
			"Bearer good-token",
			allowedID,
			"resource_delete",
			nil,
			permissions.ErrPermissionDenied,
		},
		{
			"check unauthorized token",
			permissions.Config{
				URL: srv.URL,
			},
			nil,
			"Bearer bad-token",
			allowedID,
			"resource_create",
			nil,
			permissions.ErrBadResponse,
		},
		{
			"check invalid token (too short)",
			permissions.Config{
				URL: srv.URL,
			},
			nil,
			"short",
			allowedID,
			"resource_create",
			permissions.ErrInvalidAuthToken,
			nil,
		},
		{
			"check invalid token (not bearer)",
			permissions.Config{
				URL: srv.URL,
			},
			nil,
			"not-bearer some token",
			allowedID,
			"resource_create",
			permissions.ErrInvalidAuthToken,
			nil,
		},
		{
			"check denied resource",
			permissions.Config{
				URL: srv.URL,
			},
			nil,
			"Bearer good-token",
			deniedID,
			"resource_create",
			nil,
			permissions.ErrPermissionDenied,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			perms, err := permissions.New(tc.config, tc.options...)

			require.NoError(t, err)

			resp := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "/", nil)

			if tc.authHeader != "" {
				req.Header.Set("Authorization", tc.authHeader)
			}

			engine := echo.New()

			ctx := engine.NewContext(req, resp)

			ctx.Set(echojwtx.ActorKey, tc.resource.String())

			var nextCalled bool

			nextFn := func(c echo.Context) error {
				nextCalled = true

				return nil
			}

			err = perms.Middleware()(nextFn)(ctx)

			if tc.expectMiddlewareError != nil {
				require.Error(t, err, "expected error to be returned by middleware")
				require.ErrorIs(t, err, tc.expectMiddlewareError, "unexpected error returned by middleware")
				require.False(t, nextCalled, "next should not have been called if middleware had an error")

				return
			}

			require.NoError(t, err, "unexpected error from middleware")
			require.True(t, nextCalled, "next should have been called if no error was returned")

			err = permissions.CheckAccess(ctx.Request().Context(), tc.resource, tc.action)

			if tc.expectCheckError != nil {
				require.Error(t, err, "expected error to be returned from permissions CheckAccess")
				require.ErrorIs(t, err, tc.expectCheckError, "unexpected error returned by CheckAccess")

				return
			}

			require.NoError(t, err, "unexpected error from CheckAccess")
		})
	}
}
