package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.infratographer.com/x/echojwtx"
	"go.infratographer.com/x/gidx"

	"go.infratographer.com/permissions-api/internal/query"
	"go.infratographer.com/permissions-api/internal/query/mock"
	"go.infratographer.com/permissions-api/internal/testauth"
	"go.infratographer.com/permissions-api/internal/testingx"
	"go.infratographer.com/permissions-api/internal/types"
)

func TestAssignmentCreate(t *testing.T) {
	ctx := context.Background()

	authsrv := testauth.NewServer(t)

	type testInput struct {
		path string
		json interface{}
	}

	testCases := []testingx.TestCase[testInput, *httptest.ResponseRecorder]{
		{
			Name: "BadSubjectID",
			Input: testInput{
				path: "/api/v1/roles/permrol-abc123/assignments",
				json: map[string]interface{}{
					"subject_id": "bad-id",
				},
			},
			SetupFn: func(ctx context.Context, _ *testing.T) context.Context {
				engine := mock.Engine{
					Namespace: "test",
				}

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
			Name: "InvalidRoleID",
			Input: testInput{
				path: "/api/v1/roles/bad-id/assignments",
				json: map[string]interface{}{
					"subject_id": "idntusr-abc123",
				},
			},
			SetupFn: func(ctx context.Context, _ *testing.T) context.Context {
				engine := mock.Engine{
					Namespace: "test",
				}

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
			Name: "RoleNotFound",
			Input: testInput{
				path: "/api/v1/roles/permrol-abc123/assignments",
				json: map[string]interface{}{
					"subject_id": "idntusr-def456",
				},
			},
			SetupFn: func(ctx context.Context, _ *testing.T) context.Context {
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
			Name: "Assigned",
			Input: testInput{
				path: "/api/v1/roles/permrol-abc123/assignments",
				json: map[string]interface{}{
					"subject_id": "idntusr-def456",
				},
			},
			SetupFn: func(ctx context.Context, _ *testing.T) context.Context {
				engine := mock.Engine{
					Namespace: "test",
				}

				engine.On("GetRoleResource").Return(types.Resource{}, nil)
				engine.On("SubjectHasPermission").Return(nil)
				engine.On("AssignSubjectRole").Return(nil)

				return context.WithValue(ctx, contextKeyEngine, &engine)
			},
			CheckFn: func(ctx context.Context, t *testing.T, res testingx.TestResult[*httptest.ResponseRecorder]) {
				engine := ctx.Value(contextKeyEngine).(*mock.Engine)
				engine.AssertExpectations(t)

				require.NoError(t, res.Err)
				require.NotNil(t, res.Success)

				resp := res.Success.Result()

				defer resp.Body.Close() //nolint:errcheck

				var retResp createAssignmentResponse

				err := json.NewDecoder(resp.Body).Decode(&retResp)

				require.NoError(t, err)

				assert.Equal(t, http.StatusCreated, resp.StatusCode)
				assert.True(t, retResp.Success)
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

		req, err := http.NewRequestWithContext(ctx, http.MethodPost, input.path, &body)
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

func TestAssignmentsList(t *testing.T) {
	ctx := context.Background()

	authsrv := testauth.NewServer(t)

	testCases := []testingx.TestCase[string, *httptest.ResponseRecorder]{
		{
			Name:  "RoleResourceNotFound",
			Input: "/api/v1/roles/permrol-abc123/assignments",
			SetupFn: func(ctx context.Context, _ *testing.T) context.Context {
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
			Name:  "AssignmentsRetrieved",
			Input: "/api/v1/roles/permrol-abc123/assignments",
			SetupFn: func(ctx context.Context, _ *testing.T) context.Context {
				engine := mock.Engine{
					Namespace: "test",
				}

				engine.On("GetRoleResource").Return(types.Resource{}, nil)
				engine.On("SubjectHasPermission").Return(nil)
				engine.On("ListAssignments").Return([]types.Resource{{
					ID: gidx.MustNewID("idntusr"),
				}}, nil)

				return context.WithValue(ctx, contextKeyEngine, &engine)
			},
			CheckFn: func(ctx context.Context, t *testing.T, res testingx.TestResult[*httptest.ResponseRecorder]) {
				engine := ctx.Value(contextKeyEngine).(*mock.Engine)
				engine.AssertExpectations(t)

				require.NoError(t, res.Err)
				require.NotNil(t, res.Success)

				resp := res.Success.Result()

				defer resp.Body.Close() //nolint:errcheck

				var ret listAssignmentsResponse

				err := json.NewDecoder(resp.Body).Decode(&ret)

				require.NoError(t, err)

				assert.Equal(t, http.StatusOK, resp.StatusCode)
				require.NotEmpty(t, ret.Data)
				assert.True(t, strings.HasPrefix(ret.Data[0].SubjectID, "idntusr-"))
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

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, path, nil)
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

func TestAssignmentDelete(t *testing.T) {
	ctx := context.Background()

	authsrv := testauth.NewServer(t)

	type testInput struct {
		path string
		json interface{}
	}

	testCases := []testingx.TestCase[testInput, *httptest.ResponseRecorder]{
		{
			Name: "BadSubjectID",
			Input: testInput{
				path: "/api/v1/roles/permrol-abc123/assignments",
				json: map[string]interface{}{
					"subject_id": "bad-id",
				},
			},
			SetupFn: func(ctx context.Context, _ *testing.T) context.Context {
				engine := mock.Engine{
					Namespace: "test",
				}

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
			Name: "InvalidRoleID",
			Input: testInput{
				path: "/api/v1/roles/bad-id/assignments",
				json: map[string]interface{}{
					"subject_id": "idntusr-abc123",
				},
			},
			SetupFn: func(ctx context.Context, _ *testing.T) context.Context {
				engine := mock.Engine{
					Namespace: "test",
				}

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
			Name: "RoleNotFound",
			Input: testInput{
				path: "/api/v1/roles/permrol-abc123/assignments",
				json: map[string]interface{}{
					"subject_id": "idntusr-def456",
				},
			},
			SetupFn: func(ctx context.Context, _ *testing.T) context.Context {
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
			Name: "Unassigned",
			Input: testInput{
				path: "/api/v1/roles/permrol-abc123/assignments",
				json: map[string]interface{}{
					"subject_id": "idntusr-def456",
				},
			},
			SetupFn: func(ctx context.Context, _ *testing.T) context.Context {
				engine := mock.Engine{
					Namespace: "test",
				}

				engine.On("GetRoleResource").Return(types.Resource{}, nil)
				engine.On("SubjectHasPermission").Return(nil)
				engine.On("UnassignSubjectRole").Return(nil)

				return context.WithValue(ctx, contextKeyEngine, &engine)
			},
			CheckFn: func(ctx context.Context, t *testing.T, res testingx.TestResult[*httptest.ResponseRecorder]) {
				engine := ctx.Value(contextKeyEngine).(*mock.Engine)
				engine.AssertExpectations(t)

				require.NoError(t, res.Err)
				require.NotNil(t, res.Success)

				resp := res.Success.Result()

				defer resp.Body.Close() //nolint:errcheck

				var retResp deleteAssignmentResponse

				err := json.NewDecoder(resp.Body).Decode(&retResp)

				require.NoError(t, err)

				assert.Equal(t, http.StatusOK, resp.StatusCode)
				assert.True(t, retResp.Success)
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

		req, err := http.NewRequestWithContext(ctx, http.MethodDelete, input.path, &body)
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
