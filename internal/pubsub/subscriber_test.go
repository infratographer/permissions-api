package pubsub

import (
	"context"
	"errors"
	"fmt"
	"io"
	"sync"
	"testing"
	"time"

	"go.infratographer.com/permissions-api/internal/query/mock"
	"go.infratographer.com/permissions-api/internal/testingx"

	natsserver "github.com/nats-io/nats-server/v2/server"
	"github.com/nats-io/nats.go"
	"github.com/stretchr/testify/require"
)

type contextKey int

var (
	errNoAck = errors.New("no Ack received within time limit")
	errNak   = errors.New("received Nak")
)

const (
	startTimeout     = 5 * time.Second
	natsUsername     = "natsuser"
	natsPassword     = "natspassword"
	natsConsumerName = "worker"

	contextKeyClient contextKey = iota
	contextKeyPublisher
)

func newNATSServer(t *testing.T) *natsserver.Server {
	opts := natsserver.Options{
		Host:     "127.0.0.1",
		Port:     natsserver.RANDOM_PORT,
		Username: natsUsername,
		Password: natsPassword,
	}

	server := natsserver.New(&opts)

	go server.Start()

	t.Cleanup(func() {
		server.Shutdown()
	})

	if !server.ReadyForConnections(startTimeout) {
		require.Fail(t, "NATS server failed to start")
	}

	jsConfig := natsserver.JetStreamConfig{
		StoreDir: t.TempDir(),
	}

	err := server.EnableJetStream(&jsConfig)
	require.NoError(t, err)

	return server
}

func newNATSConn(t *testing.T, addr, name string) *nats.Conn {
	natsOpts := []nats.Option{
		nats.Name(name),
		nats.UserInfo(natsUsername, natsPassword),
	}

	nc, err := nats.Connect(addr, natsOpts...)

	require.NoError(t, err)

	t.Cleanup(func() {
		nc.Close()
	})

	return nc
}

func setupClient(t *testing.T, engine *mock.Engine, addr, testName string) *Client {
	// Create a new NATS connection
	subConn := newNATSConn(t, addr, testName)

	// Create a new client with a stream based on the test name
	config := Config{
		Name:     "subscriber",
		Stream:   testName,
		Consumer: natsConsumerName,
		Prefix:   testName,
	}

	client, err := NewClient(
		config,
		WithConn(subConn),
		WithResourceTypeNames(
			[]string{
				"loadbalancer",
			},
		),
		WithQueryEngine(engine),
	)

	require.NoError(t, err)

	err = client.Listen()
	require.NoError(t, err)

	// Update the stream's consumer to sample all Acks for observability so we can see when a message was processed
	info, err := client.js.ConsumerInfo(testName, natsConsumerName)
	require.NoError(t, err)

	cfg := info.Config
	cfg.SampleFrequency = "100"

	_, err = client.js.UpdateConsumer(testName, &cfg)
	require.NoError(t, err)

	t.Cleanup(func() {
		client.Stop() //nolint:errcheck
	})

	return client
}

func waitForAck(nc *nats.Conn, client *Client, timeout time.Duration) error {
	ackSubject := fmt.Sprintf("$JS.EVENT.METRIC.CONSUMER.ACK.%s.%s", client.stream, client.consumer)

	nakSubject := fmt.Sprintf("$JS.EVENT.ADVISORY.CONSUMER.MSG_NAKED.%s.%s", client.stream, client.consumer)

	// We should only ever receive one Ack, so we close the channel directly if we get one.
	ackCh := make(chan struct{})
	ackSub, err := nc.Subscribe(ackSubject, func(m *nats.Msg) {
		close(ackCh)
	})

	defer ackSub.Unsubscribe() //nolint:errcheck

	if err != nil {
		return err
	}

	// We may receive many Naks in a single test, so we use a sync.Once to close the channel.
	nakCh := make(chan struct{})

	var nakOnce sync.Once

	nakSub, err := nc.Subscribe(nakSubject, func(m *nats.Msg) {
		nakOnce.Do(func() {
			close(nakCh)
		})
	})

	if err != nil {
		return err
	}

	defer nakSub.Unsubscribe() //nolint:errcheck

	timer := time.NewTimer(timeout)
	defer timer.Stop()

	select {
	case <-ackCh:
		return nil
	case <-nakCh:
		return errNak
	case <-timer.C:
		return errNoAck
	}
}

func contextWithPublisher(ctx context.Context, t *testing.T, addr string) context.Context {
	pubConn := newNATSConn(t, addr, "publisher")

	return context.WithValue(ctx, contextKeyPublisher, pubConn)
}

func getContextPublisher(ctx context.Context) *nats.Conn {
	return ctx.Value(contextKeyPublisher).(*nats.Conn)
}

func getContextClient(ctx context.Context) *Client {
	return ctx.Value(contextKeyClient).(*Client)
}

func TestNATS(t *testing.T) {
	server := newNATSServer(t)
	addr := server.Addr().String()

	type testInput struct {
		subject  string
		msgBytes []byte
	}

	createBytes := []byte(`{"subject_urn": "urn:infratographer:loadbalancer:fc065394-5486-4731-93b4-2726ca7e669f", "event_type": "create", "fields": {"tenant_urn": "urn:infratographer:tenant:75c8ec25-86e8-4fa7-93e2-6167684c3fb6"}}`)
	updateBytes := []byte(`{"subject_urn": "urn:infratographer:loadbalancer:fc065394-5486-4731-93b4-2726ca7e669f", "event_type": "update", "fields": {"tenant_urn": "urn:infratographer:tenant:75c8ec25-86e8-4fa7-93e2-6167684c3fb6"}}`)
	deleteBytes := []byte(`{"subject_urn": "urn:infratographer:loadbalancer:fc065394-5486-4731-93b4-2726ca7e669f", "event_type": "delete"}`)
	unknownResourceBytes := []byte(`{"subject_urn": "urn:infratographer:badresource:fc065394-5486-4731-93b4-2726ca7e669f", "event_type": "create"}`)

	// Each of these tests works as follows:
	// - A publisher NATS connection is created
	// - A client is created with a mocked engine that has its own dedicated stream and subject prefix
	// - The client's consumer is updated to emit events for all message Acks
	// - The publisher publishes a message
	// - The publisher also subscribes to JetStream events to listen for either an explicit Ack or Nak
	//
	// When writing tests, make sure the subject prefix in the test input matches the prefix provided in
	// setupClient, or else you will get undefined, racy behavior.
	testCases := []testingx.TestCase[testInput, *Client]{
		{
			Name: "goodcreate",
			Input: testInput{
				subject:  "goodcreate.loadbalancer.create",
				msgBytes: createBytes,
			},
			SetupFn: func(ctx context.Context, t *testing.T) context.Context {
				ctx = contextWithPublisher(ctx, t, addr)

				var engine mock.Engine
				engine.On("CreateRelationships").Return("", nil)

				client := setupClient(t, &engine, addr, "goodcreate")

				return context.WithValue(ctx, contextKeyClient, client)
			},
			CheckFn: func(ctx context.Context, t *testing.T, result testingx.TestResult[*Client]) {
				require.NoError(t, result.Err)

				engine := result.Success.qe.(*mock.Engine)
				engine.AssertExpectations(t)
			},
		},
		{
			Name: "errcreate",
			Input: testInput{
				subject:  "errcreate.loadbalancer.create",
				msgBytes: createBytes,
			},
			SetupFn: func(ctx context.Context, t *testing.T) context.Context {
				ctx = contextWithPublisher(ctx, t, addr)

				var engine mock.Engine
				engine.On("CreateRelationships").Return("", io.ErrUnexpectedEOF)

				client := setupClient(t, &engine, addr, "errcreate")

				return context.WithValue(ctx, contextKeyClient, client)
			},
			CheckFn: func(ctx context.Context, t *testing.T, result testingx.TestResult[*Client]) {
				require.ErrorIs(t, result.Err, errNak)
			},
		},
		{
			Name: "goodupdate",
			Input: testInput{
				subject:  "goodupdate.loadbalancer.update",
				msgBytes: updateBytes,
			},
			SetupFn: func(ctx context.Context, t *testing.T) context.Context {
				ctx = contextWithPublisher(ctx, t, addr)

				var engine mock.Engine
				engine.On("DeleteRelationships").Return("", nil)
				engine.On("CreateRelationships").Return("", nil)

				client := setupClient(t, &engine, addr, "goodupdate")

				return context.WithValue(ctx, contextKeyClient, client)
			},
			CheckFn: func(ctx context.Context, t *testing.T, result testingx.TestResult[*Client]) {
				require.NoError(t, result.Err)

				engine := result.Success.qe.(*mock.Engine)
				engine.AssertExpectations(t)
			},
		},
		{
			Name: "gooddelete",
			Input: testInput{
				subject:  "gooddelete.loadbalancer.delete",
				msgBytes: deleteBytes,
			},
			SetupFn: func(ctx context.Context, t *testing.T) context.Context {
				ctx = contextWithPublisher(ctx, t, addr)

				var engine mock.Engine
				engine.Namespace = "gooddelete"
				engine.On("DeleteRelationships").Return("", nil)

				client := setupClient(t, &engine, addr, "gooddelete")

				return context.WithValue(ctx, contextKeyClient, client)
			},
			CheckFn: func(ctx context.Context, t *testing.T, result testingx.TestResult[*Client]) {
				require.NoError(t, result.Err)

				engine := result.Success.qe.(*mock.Engine)
				engine.AssertExpectations(t)
			},
		},
		{
			Name: "badresource",
			Input: testInput{
				subject:  "badresource.fakeresource.create",
				msgBytes: unknownResourceBytes,
			},
			SetupFn: func(ctx context.Context, t *testing.T) context.Context {
				ctx = contextWithPublisher(ctx, t, addr)

				var engine mock.Engine

				client := setupClient(t, &engine, addr, "badresource")

				return context.WithValue(ctx, contextKeyClient, client)
			},
			CheckFn: func(ctx context.Context, t *testing.T, result testingx.TestResult[*Client]) {
				require.ErrorIs(t, result.Err, errNoAck)
			},
		},
	}

	testFn := func(ctx context.Context, input testInput) testingx.TestResult[*Client] {
		pubConn := getContextPublisher(ctx)
		client := getContextClient(ctx)

		js, err := pubConn.JetStream()
		if err != nil {
			return testingx.TestResult[*Client]{
				Err: err,
			}
		}

		_, err = js.PublishAsync(input.subject, input.msgBytes)
		if err != nil {
			return testingx.TestResult[*Client]{
				Err: err,
			}
		}

		err = waitForAck(pubConn, client, 3*time.Second)
		if err != nil {
			return testingx.TestResult[*Client]{
				Err: err,
			}
		}

		return testingx.TestResult[*Client]{
			Success: client,
		}
	}

	testingx.RunTests(context.Background(), t, testCases, testFn)
}
