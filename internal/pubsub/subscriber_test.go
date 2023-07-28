package pubsub

import (
	"context"
	"io"
	"testing"
	"time"

	"go.infratographer.com/permissions-api/internal/query"
	"go.infratographer.com/permissions-api/internal/query/mock"
	"go.infratographer.com/permissions-api/internal/testingx"
	"go.infratographer.com/x/events"
	"go.infratographer.com/x/gidx"
	"go.infratographer.com/x/testing/eventtools"

	"github.com/stretchr/testify/require"
)

var contextKeyEngine = struct{}{}

func setupEvents(t *testing.T, engine query.Engine) (*eventtools.TestNats, events.AuthRelationshipPublisher, *Subscriber) {
	ctx := context.Background()

	nats, err := eventtools.NewNatsServer()

	require.NoError(t, err)

	eventHandler, err := events.NewNATSConnection(nats.Config.NATS)

	require.NoError(t, err)

	subscriber, err := NewSubscriber(ctx, eventHandler, engine)

	require.NoError(t, err)

	t.Cleanup(func() {
		nats.Close()
		eventHandler.Shutdown(ctx) //nolint:errcheck
	})

	return nats, eventHandler, subscriber
}

func TestNATS(t *testing.T) {
	type testInput struct {
		subject string
		request events.AuthRelationshipRequest
	}

	createMsg := events.AuthRelationshipRequest{
		Action:   events.WriteAuthRelationshipAction,
		ObjectID: gidx.PrefixedID("loadbal-UCN7pxJO57BV_5pNiV95B"),
		Relations: []events.AuthRelationshipRelation{
			{
				Relation:  "owner",
				SubjectID: gidx.PrefixedID("tnntten-gd6RExwAz353UqHLzjC1n"),
			},
		},
	}

	noCreateMsg := events.AuthRelationshipRequest{
		Action:   events.WriteAuthRelationshipAction,
		ObjectID: gidx.PrefixedID("loadbal-EA8CJagJPM4J-yw6_skd1"),
		Relations: []events.AuthRelationshipRelation{
			{
				Relation: "owner",
			},
		},
	}

	deleteMsg := events.AuthRelationshipRequest{
		Action:   events.DeleteAuthRelationshipAction,
		ObjectID: gidx.PrefixedID("loadbal-UCN7pxJO57BV_5pNiV95B"),
		Relations: []events.AuthRelationshipRelation{
			{
				Relation:  "owner",
				SubjectID: gidx.PrefixedID("tnntten-gd6RExwAz353UqHLzjC1n"),
			},
		},
	}

	unknownResourceMsg := events.AuthRelationshipRequest{
		Action:   events.WriteAuthRelationshipAction,
		ObjectID: gidx.PrefixedID("baddres-BfqAzfYxtFNlpKPGYLmra"),
		Relations: []events.AuthRelationshipRelation{
			{
				Relation:  "owner",
				SubjectID: gidx.PrefixedID("tnntten-gd6RExwAz353UqHLzjC1n"),
			},
		},
	}

	// Each of these tests works as follows:
	// - A publisher connection is created
	// - A client is created with a mocked engine that has its own dedicated stream and subject prefix
	// - The client's consumer is updated to emit events for all message Acks
	// - The publisher publishes a message
	// - The publisher also subscribes to JetStream events to listen for either an explicit Ack or Nak
	//
	// When writing tests, make sure the subject prefix in the test input matches the prefix provided in
	// setupClient, or else you will get undefined, racy behavior.
	testCases := []testingx.TestCase[testInput, events.Message[events.AuthRelationshipResponse]]{
		{
			Name: "goodcreate",
			Input: testInput{
				subject: "goodcreate.loadbalancer",
				request: createMsg,
			},
			SetupFn: func(ctx context.Context, t *testing.T) context.Context {
				var engine mock.Engine
				engine.On("CreateRelationships").Return("", nil)

				return context.WithValue(ctx, contextKeyEngine, &engine)
			},
			CheckFn: func(ctx context.Context, t *testing.T, result testingx.TestResult[events.Message[events.AuthRelationshipResponse]]) {
				require.NoError(t, result.Err)
				require.NotNil(t, result.Success)
				require.Empty(t, result.Success.Message().Errors)

				engine := ctx.Value(contextKeyEngine).(*mock.Engine)
				engine.AssertExpectations(t)
			},
		},
		{
			Name: "errcreate",
			Input: testInput{
				subject: "errcreate.loadbalancer",
				request: createMsg,
			},
			SetupFn: func(ctx context.Context, t *testing.T) context.Context {
				var engine mock.Engine
				engine.On("CreateRelationships").Return("", io.ErrUnexpectedEOF)

				return context.WithValue(ctx, contextKeyEngine, &engine)
			},
			CheckFn: func(ctx context.Context, t *testing.T, result testingx.TestResult[events.Message[events.AuthRelationshipResponse]]) {
				require.NoError(t, result.Err)
				require.NotNil(t, result.Success)
				require.NotEmpty(t, result.Success.Message().Errors)
			},
		},
		{
			Name: "nocreate",
			Input: testInput{
				subject: "nocreate.loadbalancer",
				request: noCreateMsg,
			},
			SetupFn: func(ctx context.Context, t *testing.T) context.Context {
				var engine mock.Engine

				return context.WithValue(ctx, contextKeyEngine, &engine)
			},
			CheckFn: func(ctx context.Context, t *testing.T, result testingx.TestResult[events.Message[events.AuthRelationshipResponse]]) {
				require.Error(t, result.Err)
				require.ErrorIs(t, result.Err, events.ErrMissingAuthRelationshipRequestRelationSubjectID)
				require.Nil(t, result.Success)
			},
		},
		{
			Name: "gooddelete",
			Input: testInput{
				subject: "gooddelete.loadbalancer",
				request: deleteMsg,
			},
			SetupFn: func(ctx context.Context, t *testing.T) context.Context {
				var engine mock.Engine
				engine.Namespace = "gooddelete"
				engine.On("DeleteRelationships").Return("", nil)

				return context.WithValue(ctx, contextKeyEngine, &engine)
			},
			CheckFn: func(ctx context.Context, t *testing.T, result testingx.TestResult[events.Message[events.AuthRelationshipResponse]]) {
				require.NoError(t, result.Err)
				require.NotNil(t, result.Success)
				require.Empty(t, result.Success.Message().Errors)

				engine := ctx.Value(contextKeyEngine).(*mock.Engine)
				engine.AssertExpectations(t)
			},
		},
		{
			Name: "badresource",
			Input: testInput{
				subject: "badresource.fakeresource",
				request: unknownResourceMsg,
			},
			SetupFn: func(ctx context.Context, t *testing.T) context.Context {
				var engine mock.Engine

				return context.WithValue(ctx, contextKeyEngine, &engine)
			},
			CheckFn: func(ctx context.Context, t *testing.T, result testingx.TestResult[events.Message[events.AuthRelationshipResponse]]) {
				require.NoError(t, result.Err)
				require.NotNil(t, result.Success)
				require.NotEmpty(t, result.Success.Message().Errors)
			},
		},
	}

	testFn := func(ctx context.Context, input testInput) testingx.TestResult[events.Message[events.AuthRelationshipResponse]] {
		engine := ctx.Value(contextKeyEngine).(query.Engine)

		_, pub, sub := setupEvents(t, engine)

		err := sub.Subscribe("*." + input.subject)

		require.NoError(t, err)

		go func() {
			err = sub.Listen()

			require.NoError(t, err)
		}()

		// Allow time for the listener to to start
		time.Sleep(time.Second)

		require.NoError(t, err)

		resp, err := pub.PublishAuthRelationshipRequest(ctx, input.subject, input.request)

		return testingx.TestResult[events.Message[events.AuthRelationshipResponse]]{
			Err:     err,
			Success: resp,
		}
	}

	testingx.RunTests(context.Background(), t, testCases, testFn)
}
