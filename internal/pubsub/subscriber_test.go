package pubsub

import (
	"context"
	"testing"
	"time"

	"go.infratographer.com/permissions-api/internal/query"
	"go.infratographer.com/permissions-api/internal/query/mock"
	"go.infratographer.com/permissions-api/internal/testingx"
	"go.infratographer.com/x/events"
	"go.infratographer.com/x/gidx"
	"go.infratographer.com/x/testing/eventtools"
	"go.uber.org/zap"

	"github.com/stretchr/testify/require"
)

type contextKey int

const (
	contextKeySubscriber contextKey = iota
	contextKeyPublisher
)

func setupEvents(ctx context.Context, t *testing.T, engine query.Engine) (*events.Publisher, *Subscriber) {
	pubCfg, subCfg, err := eventtools.NewNatsServer()

	require.NoError(t, err)

	publisher, err := events.NewPublisher(pubCfg)

	require.NoError(t, err)

	logCfg := zap.NewProductionConfig()
	logCfg.Level.SetLevel(zap.DebugLevel)
	logger, err := logCfg.Build()

	require.NoError(t, err)

	subscriber, err := NewSubscriber(ctx, subCfg, engine, WithLogger(logger.Sugar()))

	require.NoError(t, err)

	t.Cleanup(func() {
		publisher.Close()  //nolint:errcheck
		subscriber.Close() //nolint:errcheck
	})

	return publisher, subscriber
}

func contextWithEvents(ctx context.Context, pub *events.Publisher, sub *Subscriber) context.Context {
	ctx = context.WithValue(ctx, contextKeyPublisher, pub)
	ctx = context.WithValue(ctx, contextKeySubscriber, sub)

	return ctx
}

func getContextPublisher(ctx context.Context) *events.Publisher {
	return ctx.Value(contextKeyPublisher).(*events.Publisher)
}

func getContextSubscriber(ctx context.Context) *Subscriber {
	return ctx.Value(contextKeySubscriber).(*Subscriber)
}

func TestNATS(t *testing.T) {
	type testInput struct {
		subject       string
		changeMessage events.ChangeMessage
	}

	createMsg := events.ChangeMessage{
		SubjectID: gidx.PrefixedID("loadbal-UCN7pxJO57BV_5pNiV95B"),
		EventType: string(events.CreateChangeType),
		AdditionalSubjectIDs: []gidx.PrefixedID{
			gidx.PrefixedID("tnntten-gd6RExwAz353UqHLzjC1n"),
		},
	}

	updateMsg := events.ChangeMessage{
		SubjectID: gidx.PrefixedID("loadbal-UCN7pxJO57BV_5pNiV95B"),
		EventType: string(events.UpdateChangeType),
		AdditionalSubjectIDs: []gidx.PrefixedID{
			gidx.PrefixedID("tnntten-gd6RExwAz353UqHLzjC1n"),
		},
	}

	deleteMsg := events.ChangeMessage{
		SubjectID: gidx.PrefixedID("loadbal-UCN7pxJO57BV_5pNiV95B"),
		EventType: string(events.DeleteChangeType),
	}

	unknownResourceMsg := events.ChangeMessage{
		SubjectID: gidx.PrefixedID("baddres-BfqAzfYxtFNlpKPGYLmra"),
		EventType: string(events.CreateChangeType),
	}

	_, _ = deleteMsg, unknownResourceMsg

	// Each of these tests works as follows:
	// - A publisher connection is created
	// - A client is created with a mocked engine that has its own dedicated stream and subject prefix
	// - The client's consumer is updated to emit events for all message Acks
	// - The publisher publishes a message
	// - The publisher also subscribes to JetStream events to listen for either an explicit Ack or Nak (TODO)
	//
	// When writing tests, make sure the subject prefix in the test input matches the prefix provided in
	// setupClient, or else you will get undefined, racy behavior.
	testCases := []testingx.TestCase[testInput, *Subscriber]{
		{
			Name: "goodcreate",
			Input: testInput{
				subject:       "goodcreate.loadbalancer.create",
				changeMessage: createMsg,
			},
			SetupFn: func(ctx context.Context, t *testing.T) context.Context {
				var engine mock.Engine
				engine.On("CreateRelationships").Return("", nil)

				publisher, subscriber := setupEvents(ctx, t, &engine)

				return contextWithEvents(ctx, publisher, subscriber)
			},
			CheckFn: func(ctx context.Context, t *testing.T, result testingx.TestResult[*Subscriber]) {
				require.NoError(t, result.Err)

				engine := result.Success.qe.(*mock.Engine)
				engine.AssertExpectations(t)
			},
		},
		// {
		// 	Name: "errcreate",
		// 	Input: testInput{
		// 		subject:       "errcreate.loadbalancer.create",
		// 		changeMessage: createMsg,
		// 	},
		// 	SetupFn: func(ctx context.Context, t *testing.T) context.Context {
		// 		var engine mock.Engine
		// 		engine.On("CreateRelationships").Return("", io.ErrUnexpectedEOF)

		// 		publisher, subscriber := setupEvents(ctx, t, &engine)

		// 		return contextWithEvents(ctx, publisher, subscriber)
		// 	},
		// 	CheckFn: func(ctx context.Context, t *testing.T, result testingx.TestResult[*Subscriber]) {
		// 		require.ErrorIs(t, result.Err, errNak)
		// 	},
		// },
		{
			Name: "goodupdate",
			Input: testInput{
				subject:       "goodupdate.loadbalancer.update",
				changeMessage: updateMsg,
			},
			SetupFn: func(ctx context.Context, t *testing.T) context.Context {
				var engine mock.Engine
				engine.On("DeleteRelationships").Return("", nil)
				engine.On("CreateRelationships").Return("", nil)

				publisher, subscriber := setupEvents(ctx, t, &engine)

				return contextWithEvents(ctx, publisher, subscriber)
			},
			CheckFn: func(ctx context.Context, t *testing.T, result testingx.TestResult[*Subscriber]) {
				require.NoError(t, result.Err)

				engine := result.Success.qe.(*mock.Engine)
				engine.AssertExpectations(t)
			},
		},
		{
			Name: "gooddelete",
			Input: testInput{
				subject:       "gooddelete.loadbalancer.delete",
				changeMessage: deleteMsg,
			},
			SetupFn: func(ctx context.Context, t *testing.T) context.Context {
				var engine mock.Engine
				engine.Namespace = "gooddelete"
				engine.On("DeleteRelationships").Return("", nil)

				publisher, subscriber := setupEvents(ctx, t, &engine)

				return contextWithEvents(ctx, publisher, subscriber)
			},
			CheckFn: func(ctx context.Context, t *testing.T, result testingx.TestResult[*Subscriber]) {
				require.NoError(t, result.Err)

				engine := result.Success.qe.(*mock.Engine)
				engine.AssertExpectations(t)
			},
		},
		// {
		// 	Name: "badresource",
		// 	Input: testInput{
		// 		subject:       "badresource.fakeresource.create",
		// 		changeMessage: unknownResourceMsg,
		// 	},
		// 	SetupFn: func(ctx context.Context, t *testing.T) context.Context {
		// 		var engine mock.Engine

		// 		publisher, subscriber := setupEvents(ctx, t, &engine)

		// 		return contextWithEvents(ctx, publisher, subscriber)
		// 	},
		// 	CheckFn: func(ctx context.Context, t *testing.T, result testingx.TestResult[*Subscriber]) {
		// 		require.ErrorIs(t, result.Err, errTimeout)
		// 	},
		// },
	}

	testFn := func(ctx context.Context, input testInput) testingx.TestResult[*Subscriber] {
		pub := getContextPublisher(ctx)
		sub := getContextSubscriber(ctx)

		err := sub.Subscribe(">")

		require.NoError(t, err)

		go func() {
			defer sub.Close()

			err = sub.Listen()

			require.NoError(t, err)
		}()

		err = pub.PublishChange(ctx, input.subject, input.changeMessage)

		require.NoError(t, err)

		// Allow time for the message to be received.
		// TODO: figure out why message.Message.Acked() and Nacked() don't work.
		time.Sleep(time.Second)

		return testingx.TestResult[*Subscriber]{
			Success: sub,
		}
	}

	testingx.RunTests(context.Background(), t, testCases, testFn)
}
