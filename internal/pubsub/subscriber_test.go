package pubsub

import (
	"context"
	"fmt"
	"io"
	"math/rand"
	"testing"
	"time"

	nc "github.com/nats-io/nats.go"
	"go.infratographer.com/permissions-api/internal/query"
	"go.infratographer.com/permissions-api/internal/query/mock"
	"go.infratographer.com/permissions-api/internal/testingx"
	"go.infratographer.com/x/events"
	"go.infratographer.com/x/gidx"
	"go.infratographer.com/x/testing/eventtools"

	"github.com/stretchr/testify/require"
)

const (
	charSet         = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	consumerLen     = 8
	sampleFrequency = "100"
)

var contextKeyEngine = struct{}{}

func newConsumerName() string {
	b := make([]byte, consumerLen)

	for i := range b {
		b[i] = charSet[rand.Intn(len(charSet))]
	}

	return fmt.Sprintf("consumer-%s", string(b))
}

func setupEvents(t *testing.T, engine query.Engine) (*eventtools.TestNats, *events.Publisher, *Subscriber, string) {
	nats, err := eventtools.NewNatsServer()

	require.NoError(t, err)

	publisher, err := events.NewPublisher(nats.PublisherConfig)

	require.NoError(t, err)

	consumerName := newConsumerName()

	subscriber, err := NewSubscriber(context.Background(), nats.SubscriberConfig, engine,
		WithNatsSubOpts(
			nc.ManualAck(),
			nc.AckExplicit(),
			nc.Durable(consumerName),
		),
	)

	require.NoError(t, err)

	t.Cleanup(func() {
		nats.Close()
		publisher.Close()  //nolint:errcheck
		subscriber.Close() //nolint:errcheck
	})

	return nats, publisher, subscriber, consumerName
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
				require.ErrorIs(t, result.Err, eventtools.ErrNack)
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

		nats, pub, sub, consumerName := setupEvents(t, engine)

		err := sub.Subscribe("*." + input.subject)

		require.NoError(t, err)

		go func() {
			defer sub.Close()

			err = sub.Listen()

			require.NoError(t, err)
		}()

		// Allow time for the listener to to start
		time.Sleep(time.Second)

		err = nats.SetConsumerSampleFrequency(consumerName, sampleFrequency)

		require.NoError(t, err)

		err = pub.PublishChange(ctx, input.subject, input.changeMessage)

		require.NoError(t, err)

		err = nats.WaitForAck(consumerName, time.Second)
		if err != nil {
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
