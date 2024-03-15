package api

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.infratographer.com/x/echojwtx"
	"go.infratographer.com/x/gidx"

	"go.infratographer.com/permissions-api/internal/query"
	"go.infratographer.com/permissions-api/internal/query/mock"
	"go.infratographer.com/permissions-api/internal/storage"
	"go.infratographer.com/permissions-api/internal/testauth"
	"go.infratographer.com/permissions-api/internal/testingx"
	"go.infratographer.com/permissions-api/internal/types"
)

var contextKeyEngine = struct{}{}

func TestRoleCreate(t *testing.T) {
	ctx := context.Background()

	authsrv := testauth.NewServer(t)

	type testInput struct {
		path string
		json interface{}
	}

	testCases := []testingx.TestCase[testInput, *httptest.ResponseRecorder]{
		{
			Name: "ErrInvalidAction",
			Input: testInput{
				path: "/api/v1/resources/tnntten-abc123/roles",
				json: map[string]interface{}{
					"name": "my role",
					"actions": []string{
						"action1",
						"action2",
					},
				},
			},
			SetupFn: func(ctx context.Context, t *testing.T) context.Context {
				engine := mock.Engine{
					Namespace: "test",
				}

				engine.On("SubjectHasPermission").Return(nil)
				engine.On("CreateRole").Return(types.Role{}, query.ErrInvalidAction)

				return context.WithValue(ctx, contextKeyEngine, &engine)
			},
			CheckFn: func(ctx context.Context, t *testing.T, res testingx.TestResult[*httptest.ResponseRecorder]) {
				engine := ctx.Value(contextKeyEngine).(*mock.Engine)
				engine.AssertExpectations(t)

				require.NoError(t, res.Err)
				require.NotNil(t, res.Success)

				assert.Equal(t, http.StatusBadRequest, res.Success.Code)
			},
		},
		{
			Name: "ErrRoleAlreadyExists",
			Input: testInput{
				path: "/api/v1/resources/tnntten-abc123/roles",
				json: map[string]interface{}{
					"name": "my role",
					"actions": []string{
						"action1",
						"action2",
					},
				},
			},
			SetupFn: func(ctx context.Context, t *testing.T) context.Context {
				engine := mock.Engine{
					Namespace: "test",
				}

				engine.On("SubjectHasPermission").Return(nil)
				engine.On("CreateRole").Return(types.Role{}, storage.ErrRoleAlreadyExists)

				return context.WithValue(ctx, contextKeyEngine, &engine)
			},
			CheckFn: func(ctx context.Context, t *testing.T, res testingx.TestResult[*httptest.ResponseRecorder]) {
				engine := ctx.Value(contextKeyEngine).(*mock.Engine)
				engine.AssertExpectations(t)

				require.NoError(t, res.Err)
				require.NotNil(t, res.Success)

				assert.Equal(t, http.StatusConflict, res.Success.Code)
			},
		},
		{
			Name: "ErrRoleNameTaken",
			Input: testInput{
				path: "/api/v1/resources/tnntten-abc123/roles",
				json: map[string]interface{}{
					"name": "my role",
					"actions": []string{
						"action1",
						"action2",
					},
				},
			},
			SetupFn: func(ctx context.Context, t *testing.T) context.Context {
				engine := mock.Engine{
					Namespace: "test",
				}

				engine.On("SubjectHasPermission").Return(nil)
				engine.On("CreateRole").Return(types.Role{}, storage.ErrRoleNameTaken)

				return context.WithValue(ctx, contextKeyEngine, &engine)
			},
			CheckFn: func(ctx context.Context, t *testing.T, res testingx.TestResult[*httptest.ResponseRecorder]) {
				engine := ctx.Value(contextKeyEngine).(*mock.Engine)
				engine.AssertExpectations(t)

				require.NoError(t, res.Err)
				require.NotNil(t, res.Success)

				assert.Equal(t, http.StatusConflict, res.Success.Code)
			},
		},
		{
			Name: "RoleCreated",
			Input: testInput{
				path: "/api/v1/resources/tnntten-abc123/roles",
				json: map[string]interface{}{
					"name": t.Name(),
					"actions": []string{
						"action1",
						"action2",
					},
				},
			},
			SetupFn: func(ctx context.Context, t *testing.T) context.Context {
				engine := mock.Engine{
					Namespace: "test",
				}

				engine.On("SubjectHasPermission").Return(nil)
				engine.On("CreateRole").Return(types.Role{
					ID:   gidx.MustNewID(query.RolePrefix),
					Name: t.Name(),
					Actions: []string{
						"action1",
						"action2",
					},
					CreatedBy: "idntusr-abc123",
					UpdatedBy: "idntusr-def456",
					CreatedAt: time.Now(),
					UpdatedAt: time.Now(),
				}, nil)

				return context.WithValue(ctx, contextKeyEngine, &engine)
			},
			CheckFn: func(ctx context.Context, t *testing.T, res testingx.TestResult[*httptest.ResponseRecorder]) {
				engine := ctx.Value(contextKeyEngine).(*mock.Engine)
				engine.AssertExpectations(t)

				require.NoError(t, res.Err)
				require.NotNil(t, res.Success)

				resp := res.Success.Result()

				defer resp.Body.Close()

				var role roleResponse

				err := json.NewDecoder(resp.Body).Decode(&role)

				require.NoError(t, err)

				assert.Equal(t, http.StatusCreated, resp.StatusCode)
				assert.Equal(t, query.RolePrefix, role.ID.Prefix())
				assert.Equal(t, t.Name(), role.Name)
				assert.Equal(t, []string{"action1", "action2"}, role.Actions)
				assert.Equal(t, "idntusr-abc123", role.CreatedBy.String())
				assert.Equal(t, "idntusr-def456", role.UpdatedBy.String())
				assert.NotEmpty(t, role.CreatedAt)
				assert.NotEmpty(t, role.UpdatedAt)
			},
		},
	}

	testFn := func(ctx context.Context, input testInput) testingx.TestResult[*httptest.ResponseRecorder] {
		result := testingx.TestResult[*httptest.ResponseRecorder]{}

		engine := ctx.Value(contextKeyEngine).(query.Engine)

		router, err := NewRouter(echojwtx.AuthConfig{Issuer: authsrv.Issuer}, engine)
		if err != nil {
			result.Err = err

			return result
		}

		e := echo.New()
		e.Use(echoTestLogger(t, e))

		router.Routes(e.Group(""))

		var body bytes.Buffer

		if input.json != nil {
			if err = json.NewEncoder(&body).Encode(input.json); err != nil {
				result.Err = err

				return result
			}
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodPost, "http://127.0.0.1"+input.path, &body)
		if err != nil {
			result.Err = err

			return result
		}

		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+authsrv.TSignSubject(t, "idntusr-abc123"))

		resp := httptest.NewRecorder()

		e.ServeHTTP(resp, req)

		result.Success = resp

		return result
	}

	testingx.RunTests(ctx, t, testCases, testFn)
}

func TestRoleUpdate(t *testing.T) {
	ctx := context.Background()

	authsrv := testauth.NewServer(t)

	type testInput struct {
		path string
		json interface{}
	}

	testCases := []testingx.TestCase[testInput, *httptest.ResponseRecorder]{
		{
			Name: "ErrInvalidAction",
			Input: testInput{
				path: "/api/v1/roles/permrol-abc123",
				json: map[string]interface{}{
					"actions": []string{
						"action1",
						"action2",
					},
				},
			},
			SetupFn: func(ctx context.Context, t *testing.T) context.Context {
				engine := mock.Engine{
					Namespace: "test",
				}

				engine.On("GetRoleResource").Return(types.Resource{}, nil)
				engine.On("SubjectHasPermission").Return(nil)
				engine.On("UpdateRole").Return(types.Role{}, query.ErrInvalidAction)

				return context.WithValue(ctx, contextKeyEngine, &engine)
			},
			CheckFn: func(ctx context.Context, t *testing.T, res testingx.TestResult[*httptest.ResponseRecorder]) {
				engine := ctx.Value(contextKeyEngine).(*mock.Engine)
				engine.AssertExpectations(t)

				require.NoError(t, res.Err)
				require.NotNil(t, res.Success)

				assert.Equal(t, http.StatusBadRequest, res.Success.Code)
			},
		},
		{
			Name: "ErrRoleNameTaken",
			Input: testInput{
				path: "/api/v1/roles/permrol-abc123",
				json: map[string]interface{}{
					"name": "my role",
					"actions": []string{
						"action1",
						"action2",
					},
				},
			},
			SetupFn: func(ctx context.Context, t *testing.T) context.Context {
				engine := mock.Engine{
					Namespace: "test",
				}

				engine.On("GetRoleResource").Return(types.Resource{}, nil)
				engine.On("SubjectHasPermission").Return(nil)
				engine.On("UpdateRole").Return(types.Role{}, storage.ErrRoleNameTaken)

				return context.WithValue(ctx, contextKeyEngine, &engine)
			},
			CheckFn: func(ctx context.Context, t *testing.T, res testingx.TestResult[*httptest.ResponseRecorder]) {
				engine := ctx.Value(contextKeyEngine).(*mock.Engine)
				engine.AssertExpectations(t)

				require.NoError(t, res.Err)
				require.NotNil(t, res.Success)

				assert.Equal(t, http.StatusConflict, res.Success.Code)
			},
		},
		{
			Name: "RoleResourceNotFound",
			Input: testInput{
				path: "/api/v1/roles/permrol-abc123",
				json: map[string]interface{}{
					"name": "my role",
					"actions": []string{
						"action1",
						"action2",
					},
				},
			},
			SetupFn: func(ctx context.Context, t *testing.T) context.Context {
				engine := mock.Engine{
					Namespace: "test",
				}

				engine.On("GetRoleResource").Return(types.Resource{}, query.ErrRoleNotFound)

				return context.WithValue(ctx, contextKeyEngine, &engine)
			},
			CheckFn: func(ctx context.Context, t *testing.T, res testingx.TestResult[*httptest.ResponseRecorder]) {
				engine := ctx.Value(contextKeyEngine).(*mock.Engine)
				engine.AssertExpectations(t)

				require.NoError(t, res.Err)
				require.NotNil(t, res.Success)

				assert.Equal(t, http.StatusNotFound, res.Success.Code)
			},
		},
		{
			Name: "RoleUpdated",
			Input: testInput{
				path: "/api/v1/roles/permrol-abc123",
				json: map[string]interface{}{
					"name": t.Name(),
					"actions": []string{
						"action1",
						"action2",
					},
				},
			},
			SetupFn: func(ctx context.Context, t *testing.T) context.Context {
				engine := mock.Engine{
					Namespace: "test",
				}

				engine.On("GetRoleResource").Return(types.Resource{}, nil)
				engine.On("SubjectHasPermission").Return(nil)
				engine.On("UpdateRole").Return(types.Role{
					ID:   gidx.MustNewID(query.RolePrefix),
					Name: t.Name(),
					Actions: []string{
						"action1",
						"action2",
					},
					CreatedBy: "idntusr-abc123",
					UpdatedBy: "idntusr-def456",
					CreatedAt: time.Now(),
					UpdatedAt: time.Now(),
				}, nil)

				return context.WithValue(ctx, contextKeyEngine, &engine)
			},
			CheckFn: func(ctx context.Context, t *testing.T, res testingx.TestResult[*httptest.ResponseRecorder]) {
				engine := ctx.Value(contextKeyEngine).(*mock.Engine)
				engine.AssertExpectations(t)

				require.NoError(t, res.Err)
				require.NotNil(t, res.Success)

				resp := res.Success.Result()

				defer resp.Body.Close()

				var role roleResponse

				err := json.NewDecoder(resp.Body).Decode(&role)

				require.NoError(t, err)

				assert.Equal(t, http.StatusOK, resp.StatusCode)
				assert.Equal(t, query.RolePrefix, role.ID.Prefix())
				assert.Equal(t, t.Name(), role.Name)
				assert.Equal(t, []string{"action1", "action2"}, role.Actions)
				assert.Equal(t, "idntusr-abc123", role.CreatedBy.String())
				assert.Equal(t, "idntusr-def456", role.UpdatedBy.String())
				assert.NotEmpty(t, role.CreatedAt)
				assert.NotEmpty(t, role.UpdatedAt)
			},
		},
	}

	testFn := func(ctx context.Context, input testInput) testingx.TestResult[*httptest.ResponseRecorder] {
		result := testingx.TestResult[*httptest.ResponseRecorder]{}

		engine := ctx.Value(contextKeyEngine).(query.Engine)

		router, err := NewRouter(echojwtx.AuthConfig{Issuer: authsrv.Issuer}, engine)
		if err != nil {
			result.Err = err

			return result
		}

		e := echo.New()
		e.Use(echoTestLogger(t, e))

		router.Routes(e.Group(""))

		var body bytes.Buffer

		if input.json != nil {
			if err = json.NewEncoder(&body).Encode(input.json); err != nil {
				result.Err = err

				return result
			}
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodPatch, "http://127.0.0.1"+input.path, &body)
		if err != nil {
			result.Err = err

			return result
		}

		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+authsrv.TSignSubject(t, "idntusr-abc123"))

		resp := httptest.NewRecorder()

		e.ServeHTTP(resp, req)

		result.Success = resp

		return result
	}

	testingx.RunTests(ctx, t, testCases, testFn)
}

func TestRoleGet(t *testing.T) {
	ctx := context.Background()

	authsrv := testauth.NewServer(t)

	testCases := []testingx.TestCase[string, *httptest.ResponseRecorder]{
		{
			Name:  "RoleResourceNotFound",
			Input: "/api/v1/roles/permrol-abc123",
			SetupFn: func(ctx context.Context, t *testing.T) context.Context {
				engine := mock.Engine{
					Namespace: "test",
				}

				engine.On("GetRoleResource").Return(types.Resource{}, query.ErrRoleNotFound)

				return context.WithValue(ctx, contextKeyEngine, &engine)
			},
			CheckFn: func(ctx context.Context, t *testing.T, res testingx.TestResult[*httptest.ResponseRecorder]) {
				engine := ctx.Value(contextKeyEngine).(*mock.Engine)
				engine.AssertExpectations(t)

				require.NoError(t, res.Err)
				require.NotNil(t, res.Success)

				assert.Equal(t, http.StatusNotFound, res.Success.Code)
			},
		},
		{
			Name:  "RoleRetrieved",
			Input: "/api/v1/roles/permrol-abc123",
			SetupFn: func(ctx context.Context, t *testing.T) context.Context {
				engine := mock.Engine{
					Namespace: "test",
				}

				engine.On("GetRoleResource").Return(types.Resource{}, nil)
				engine.On("SubjectHasPermission").Return(nil)
				engine.On("GetRole").Return(types.Role{
					ID:   gidx.MustNewID(query.RolePrefix),
					Name: t.Name(),
					Actions: []string{
						"action1",
						"action2",
					},
					CreatedBy: "idntusr-abc123",
					UpdatedBy: "idntusr-def456",
					CreatedAt: time.Now(),
					UpdatedAt: time.Now(),
				}, nil)

				return context.WithValue(ctx, contextKeyEngine, &engine)
			},
			CheckFn: func(ctx context.Context, t *testing.T, res testingx.TestResult[*httptest.ResponseRecorder]) {
				engine := ctx.Value(contextKeyEngine).(*mock.Engine)
				engine.AssertExpectations(t)

				require.NoError(t, res.Err)
				require.NotNil(t, res.Success)

				resp := res.Success.Result()

				defer resp.Body.Close()

				var role roleResponse

				err := json.NewDecoder(resp.Body).Decode(&role)

				require.NoError(t, err)

				assert.Equal(t, http.StatusOK, resp.StatusCode)
				assert.Equal(t, query.RolePrefix, role.ID.Prefix())
				assert.Equal(t, t.Name(), role.Name)
				assert.Equal(t, []string{"action1", "action2"}, role.Actions)
				assert.Equal(t, "idntusr-abc123", role.CreatedBy.String())
				assert.Equal(t, "idntusr-def456", role.UpdatedBy.String())
				assert.NotEmpty(t, role.CreatedAt)
				assert.NotEmpty(t, role.UpdatedAt)
			},
		},
	}

	testFn := func(ctx context.Context, path string) testingx.TestResult[*httptest.ResponseRecorder] {
		result := testingx.TestResult[*httptest.ResponseRecorder]{}

		engine := ctx.Value(contextKeyEngine).(query.Engine)

		router, err := NewRouter(echojwtx.AuthConfig{Issuer: authsrv.Issuer}, engine)
		if err != nil {
			result.Err = err

			return result
		}

		e := echo.New()
		e.Use(echoTestLogger(t, e))

		router.Routes(e.Group(""))

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://127.0.0.1"+path, nil)
		if err != nil {
			result.Err = err

			return result
		}

		req.Header.Set("Authorization", "Bearer "+authsrv.TSignSubject(t, "idntusr-abc123"))

		resp := httptest.NewRecorder()

		e.ServeHTTP(resp, req)

		result.Success = resp

		return result
	}

	testingx.RunTests(ctx, t, testCases, testFn)
}

func TestRoleDelete(t *testing.T) {
	ctx := context.Background()

	authsrv := testauth.NewServer(t)

	testCases := []testingx.TestCase[string, *httptest.ResponseRecorder]{
		{
			Name:  "UnknownError",
			Input: "/api/v1/roles/permrol-abc123",
			SetupFn: func(ctx context.Context, t *testing.T) context.Context {
				engine := mock.Engine{
					Namespace: "test",
				}

				engine.On("GetRoleResource").Return(types.Resource{}, nil)
				engine.On("SubjectHasPermission").Return(nil)
				engine.On("DeleteRole").Return(io.EOF)

				return context.WithValue(ctx, contextKeyEngine, &engine)
			},
			CheckFn: func(ctx context.Context, t *testing.T, res testingx.TestResult[*httptest.ResponseRecorder]) {
				engine := ctx.Value(contextKeyEngine).(*mock.Engine)
				engine.AssertExpectations(t)

				require.NoError(t, res.Err)
				require.NotNil(t, res.Success)

				assert.Equal(t, http.StatusInternalServerError, res.Success.Code)
			},
		},
		{
			Name:  "RoleResourceNotFound",
			Input: "/api/v1/roles/permrol-abc123",
			SetupFn: func(ctx context.Context, t *testing.T) context.Context {
				engine := mock.Engine{
					Namespace: "test",
				}

				engine.On("GetRoleResource").Return(types.Resource{}, query.ErrRoleNotFound)

				return context.WithValue(ctx, contextKeyEngine, &engine)
			},
			CheckFn: func(ctx context.Context, t *testing.T, res testingx.TestResult[*httptest.ResponseRecorder]) {
				engine := ctx.Value(contextKeyEngine).(*mock.Engine)
				engine.AssertExpectations(t)

				require.NoError(t, res.Err)
				require.NotNil(t, res.Success)

				assert.Equal(t, http.StatusNotFound, res.Success.Code)
			},
		},
		{
			Name:  "RoleDeleted",
			Input: "/api/v1/roles/permrol-abc123",
			SetupFn: func(ctx context.Context, t *testing.T) context.Context {
				engine := mock.Engine{
					Namespace: "test",
				}

				engine.On("GetRoleResource").Return(types.Resource{}, nil)
				engine.On("SubjectHasPermission").Return(nil)
				engine.On("DeleteRole").Return(nil)

				return context.WithValue(ctx, contextKeyEngine, &engine)
			},
			CheckFn: func(ctx context.Context, t *testing.T, res testingx.TestResult[*httptest.ResponseRecorder]) {
				engine := ctx.Value(contextKeyEngine).(*mock.Engine)
				engine.AssertExpectations(t)

				require.NoError(t, res.Err)
				require.NotNil(t, res.Success)

				resp := res.Success.Result()

				defer resp.Body.Close()

				var roleResp deleteRoleResponse

				err := json.NewDecoder(resp.Body).Decode(&roleResp)

				require.NoError(t, err)

				assert.Equal(t, http.StatusOK, resp.StatusCode)
				assert.True(t, roleResp.Success)
			},
		},
	}

	testFn := func(ctx context.Context, path string) testingx.TestResult[*httptest.ResponseRecorder] {
		result := testingx.TestResult[*httptest.ResponseRecorder]{}

		engine := ctx.Value(contextKeyEngine).(query.Engine)

		router, err := NewRouter(echojwtx.AuthConfig{Issuer: authsrv.Issuer}, engine)
		if err != nil {
			result.Err = err

			return result
		}

		e := echo.New()
		e.Use(echoTestLogger(t, e))

		router.Routes(e.Group(""))

		req, err := http.NewRequestWithContext(ctx, http.MethodDelete, "http://127.0.0.1"+path, nil)
		if err != nil {
			result.Err = err

			return result
		}

		req.Header.Set("Authorization", "Bearer "+authsrv.TSignSubject(t, "idntusr-abc123"))

		resp := httptest.NewRecorder()

		e.ServeHTTP(resp, req)

		result.Success = resp

		return result
	}

	testingx.RunTests(ctx, t, testCases, testFn)
}

func echoTestLogger(t *testing.T, e *echo.Echo) echo.MiddlewareFunc {
	t.Helper()

	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			err := next(c)

			req := c.Request()

			if err == nil {
				t.Logf("%s %s", req.Method, req.URL.String())

				return nil
			}

			t.Logf("%s %s: %s", req.Method, req.URL.String(), err.Error())

			return err
		}
	}
}
