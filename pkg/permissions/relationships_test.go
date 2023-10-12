package permissions_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/require"
	"go.infratographer.com/x/events"
	"go.infratographer.com/x/gidx"
	"go.infratographer.com/x/testing/eventtools"

	"go.infratographer.com/permissions-api/pkg/permissions"
)

func TestMiddlewareMissing(t *testing.T) {
	ctx := context.Background()

	err := permissions.CreateAuthRelationships(ctx, "test", gidx.NullPrefixedID)
	require.Error(t, err)
	require.ErrorIs(t, err, permissions.ErrPermissionsMiddlewareMissing)

	err = permissions.DeleteAuthRelationships(ctx, "test", gidx.NullPrefixedID)
	require.Error(t, err)
	require.ErrorIs(t, err, permissions.ErrPermissionsMiddlewareMissing)
}

func TestNoRespondersIgnore(t *testing.T) {
	t.Run("not ignored", func(t *testing.T) {
		ctx := context.Background()

		mockEvents := new(eventtools.MockConnection)

		config := permissions.Config{
			IgnoreNoResponders: false,
		}

		perms, err := permissions.New(config, permissions.WithEventsPublisher(mockEvents))
		require.NoError(t, err)

		relation := events.AuthRelationshipRelation{
			Relation:  "parent",
			SubjectID: "testten-abc",
		}

		expectRelationshipRequest := events.AuthRelationshipRequest{
			Action:   events.WriteAuthRelationshipAction,
			ObjectID: "testten-abc123",
			Relations: []events.AuthRelationshipRelation{
				relation,
			},
		}

		responseMessage := new(eventtools.MockMessage[events.AuthRelationshipResponse])

		responseMessage.On("Message").Return(events.AuthRelationshipResponse{})
		responseMessage.On("Error").Return(nil)

		mockEvents.On("PublishAuthRelationshipRequest", "test", expectRelationshipRequest).Return(responseMessage, events.ErrRequestNoResponders)

		err = perms.CreateAuthRelationships(ctx, "test", "testten-abc123", relation)
		require.Error(t, err)
		require.ErrorIs(t, err, events.ErrRequestNoResponders)

		mockEvents.AssertExpectations(t)
	})
	t.Run("ignored", func(t *testing.T) {
		ctx := context.Background()

		mockEvents := new(eventtools.MockConnection)

		config := permissions.Config{
			IgnoreNoResponders: true,
		}

		perms, err := permissions.New(config, permissions.WithEventsPublisher(mockEvents))
		require.NoError(t, err)

		relation := events.AuthRelationshipRelation{
			Relation:  "parent",
			SubjectID: "testten-abc",
		}

		expectRelationshipRequest := events.AuthRelationshipRequest{
			Action:   events.WriteAuthRelationshipAction,
			ObjectID: "testten-abc123",
			Relations: []events.AuthRelationshipRelation{
				relation,
			},
		}

		responseMessage := new(eventtools.MockMessage[events.AuthRelationshipResponse])

		mockEvents.On("PublishAuthRelationshipRequest", "test", expectRelationshipRequest).Return(responseMessage, events.ErrRequestNoResponders)

		err = perms.CreateAuthRelationships(ctx, "test", "testten-abc123", relation)
		require.NoError(t, err)

		mockEvents.AssertExpectations(t)
	})
}

func TestRelationshipCreate(t *testing.T) {
	testCases := []struct {
		name   string
		events bool

		resourceID gidx.PrefixedID
		relation   string
		subjectID  gidx.PrefixedID

		expectRequest *events.AuthRelationshipRequest

		responseErrors []error

		expectError error
	}{
		{
			"no events",
			false,
			"testten-abc123",
			"parent",
			"testten-abc",
			nil,
			nil,
			nil,
		},
		{
			"missing resourceID",
			true,
			"",
			"relation",
			"subject",
			nil,
			nil,
			events.ErrMissingAuthRelationshipRequestObjectID,
		},
		{
			"missing relation",
			true,
			"resource",
			"",
			"subject",
			nil,
			nil,
			events.ErrMissingAuthRelationshipRequestRelationRelation,
		},
		{
			"missing subject",
			true,
			"resource",
			"relation",
			"",
			nil,
			nil,
			events.ErrMissingAuthRelationshipRequestRelationSubjectID,
		},
		{
			"success",
			true,
			"testten-abc123",
			"parent",
			"testten-abc",
			&events.AuthRelationshipRequest{
				Action:   events.WriteAuthRelationshipAction,
				ObjectID: "testten-abc123",
				Relations: []events.AuthRelationshipRelation{
					{
						Relation:  "parent",
						SubjectID: "testten-abc",
					},
				},
			},
			nil,
			nil,
		},
		{
			"response errors are returned",
			true,
			"testten-abc123",
			"parent",
			"testten-abc",
			&events.AuthRelationshipRequest{
				Action:   events.WriteAuthRelationshipAction,
				ObjectID: "testten-abc123",
				Relations: []events.AuthRelationshipRelation{
					{
						Relation:  "parent",
						SubjectID: "testten-abc",
					},
				},
			},
			[]error{events.ErrProviderNotConfigured},
			events.ErrProviderNotConfigured,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mockEvents := new(eventtools.MockConnection)

			var options []permissions.Option

			if tc.events {
				options = append(options, permissions.WithEventsPublisher(mockEvents))
			}

			perms, err := permissions.New(permissions.Config{}, options...)

			require.NoError(t, err)

			resp := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "/", nil)

			engine := echo.New()

			ctx := engine.NewContext(req, resp)

			var nextCalled bool

			nextFn := func(c echo.Context) error {
				nextCalled = true

				return nil
			}

			err = perms.Middleware()(nextFn)(ctx)

			require.NoError(t, err)

			require.True(t, nextCalled, "next should have been called")

			if tc.expectRequest != nil {
				response := events.AuthRelationshipResponse{
					Errors: tc.responseErrors,
				}

				respMsg := new(eventtools.MockMessage[events.AuthRelationshipResponse])

				respMsg.On("Message").Return(response, nil)
				respMsg.On("Error").Return(nil)

				mockEvents.On("PublishAuthRelationshipRequest", "test", *tc.expectRequest).Return(respMsg, nil)
			}

			relation := events.AuthRelationshipRelation{
				Relation:  tc.relation,
				SubjectID: tc.subjectID,
			}

			err = perms.CreateAuthRelationships(ctx.Request().Context(), "test", tc.resourceID, relation)

			mockEvents.AssertExpectations(t)

			if tc.expectError != nil {
				require.Error(t, err, "expected error to be returned")
				require.ErrorIs(t, err, tc.expectError, "unexpected error returned")

				return
			}

			require.NoError(t, err)
		})
	}
}
