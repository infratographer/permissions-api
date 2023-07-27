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

const (
	sampleFrequency = "100"
)

var contextKeyEngine = struct{}{}

func setupEvents(t *testing.T, engine query.Engine) (*eventtools.TestNats, events.Publisher, *Subscriber) {
	ctx := context.Background()

	nats, err := eventtools.NewNatsServer()

	require.NoError(t, err)

	publisher, err := events.NewNATSConnection(nats.Config.NATS)

	require.NoError(t, err)

	subscriber, err := NewSubscriber(ctx, nats.Config, engine)

	require.NoError(t, err)

	t.Cleanup(func() {
		nats.Close()
		publisher.Shutdown(ctx)  //nolint:errcheck
		subscriber.Shutdown(ctx) //nolint:errcheck
	})

	return nats, publisher, subscriber
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
			gidx.PrefixedID("othrsid-kXboa2UZbaNzMhng9vVha"),
			gidx.PrefixedID("tnntten-gd6RExwAz353UqHLzjC1n"),
		},
	}

	noCreateMsg := events.ChangeMessage{
		SubjectID: gidx.PrefixedID("loadbal-EA8CJagJPM4J-yw6_skd1"),
		EventType: string(events.CreateChangeType),
		AdditionalSubjectIDs: []gidx.PrefixedID{
			gidx.PrefixedID("othrsid-kXboa2UZbaNzMhng9vVha"),
		},
	}

	updateMsg := events.ChangeMessage{
		SubjectID: gidx.PrefixedID("loadbal-UCN7pxJO57BV_5pNiV95B"),
		EventType: string(events.UpdateChangeType),
		AdditionalSubjectIDs: []gidx.PrefixedID{
			gidx.PrefixedID("othrsid-kXboa2UZbaNzMhng9vVha"),
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

	// Each of these tests works as follows:
	// - A publisher connection is created
	// - A client is created with a mocked engine that has its own dedicated stream and subject prefix
	// - The client's consumer is updated to emit events for all message Acks
	// - The publisher publishes a message
	// - The publisher also subscribes to JetStream events to listen for either an explicit Ack or Nak
	//
	// When writing tests, make sure the subject prefix in the test input matches the prefix provided in
	// setupClient, or else you will get undefined, racy behavior.
	testCases := []testingx.TestCase[testInput, *Subscriber]{
		{
			Name: "goodcreate",
			Input: testInput{
				subject:       "goodcreate.loadbalancer",
				changeMessage: createMsg,
			},
			SetupFn: func(ctx context.Context, t *testing.T) context.Context {
				var engine mock.Engine
				engine.On("CreateRelationships").Return("", nil)

				return context.WithValue(ctx, contextKeyEngine, &engine)
			},
			CheckFn: func(ctx context.Context, t *testing.T, result testingx.TestResult[*Subscriber]) {
				require.NoError(t, result.Err)

				engine := result.Success.qe.(*mock.Engine)
				engine.AssertExpectations(t)
			},
		},
		{
			Name: "errcreate",
			Input: testInput{
				subject:       "errcreate.loadbalancer",
				changeMessage: createMsg,
			},
			SetupFn: func(ctx context.Context, t *testing.T) context.Context {
				var engine mock.Engine
				engine.On("CreateRelationships").Return("", io.ErrUnexpectedEOF)

				return context.WithValue(ctx, contextKeyEngine, &engine)
			},
			CheckFn: func(ctx context.Context, t *testing.T, result testingx.TestResult[*Subscriber]) {
				require.NoError(t, result.Err)
			},
		},
		{
			Name: "nocreate",
			Input: testInput{
				subject:       "nocreate.loadbalancer",
				changeMessage: noCreateMsg,
			},
			SetupFn: func(ctx context.Context, t *testing.T) context.Context {
				var engine mock.Engine

				return context.WithValue(ctx, contextKeyEngine, &engine)
			},
			CheckFn: func(ctx context.Context, t *testing.T, result testingx.TestResult[*Subscriber]) {
				require.NoError(t, result.Err)
			},
		},
		{
			Name: "goodupdate",
			Input: testInput{
				subject:       "goodupdate.loadbalancer",
				changeMessage: updateMsg,
			},
			SetupFn: func(ctx context.Context, t *testing.T) context.Context {
				var engine mock.Engine
				engine.On("DeleteRelationships").Return("", nil)
				engine.On("CreateRelationships").Return("", nil)

				return context.WithValue(ctx, contextKeyEngine, &engine)
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
				subject:       "gooddelete.loadbalancer",
				changeMessage: deleteMsg,
			},
			SetupFn: func(ctx context.Context, t *testing.T) context.Context {
				var engine mock.Engine
				engine.Namespace = "gooddelete"
				engine.On("DeleteRelationships").Return("", nil)

				return context.WithValue(ctx, contextKeyEngine, &engine)
			},
			CheckFn: func(ctx context.Context, t *testing.T, result testingx.TestResult[*Subscriber]) {
				require.NoError(t, result.Err)

				engine := result.Success.qe.(*mock.Engine)
				engine.AssertExpectations(t)
			},
		},
		{
			Name: "badresource",
			Input: testInput{
				subject:       "badresource.fakeresource",
				changeMessage: unknownResourceMsg,
			},
			SetupFn: func(ctx context.Context, t *testing.T) context.Context {
				var engine mock.Engine

				return context.WithValue(ctx, contextKeyEngine, &engine)
			},
			CheckFn: func(ctx context.Context, t *testing.T, result testingx.TestResult[*Subscriber]) {
				require.NoError(t, result.Err)
			},
		},
	}

	testFn := func(ctx context.Context, input testInput) testingx.TestResult[*Subscriber] {
		engine := ctx.Value(contextKeyEngine).(query.Engine)

		nats, pub, sub := setupEvents(t, engine)

		consumerName := events.NATSConsumerDurableName("", eventtools.Prefix+".changes.*."+input.subject)

		err := sub.Subscribe("*." + input.subject)

		require.NoError(t, err)

		go func() {
			err = sub.Listen()

			require.NoError(t, err)
		}()

		// Allow time for the listener to to start
		time.Sleep(time.Second)

		err = nats.SetConsumerSampleFrequency(consumerName, sampleFrequency)

		require.NoError(t, err)

		ackErr := make(chan error, 1)

		go func() {
			ackErr <- nats.WaitForAck(consumerName, 5*time.Second)
		}()

		_, err = pub.PublishChange(ctx, input.subject, input.changeMessage)

		require.NoError(t, err)

		if err = <-ackErr; err != nil {
			return testingx.TestResult[*Subscriber]{
				Err: err,
			}
		}

		return testingx.TestResult[*Subscriber]{
			Success: sub,
		}
	}

	testingx.RunTests(context.Background(), t, testCases, testFn)
}
