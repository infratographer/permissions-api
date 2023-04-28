package pubsub

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"go.infratographer.com/permissions-api/internal/query"
	"go.infratographer.com/permissions-api/internal/types"
	"go.infratographer.com/x/pubsubx"
	"go.infratographer.com/x/urnx"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"

	"github.com/nats-io/nats.go"
)

var tracer = otel.Tracer("go.infratographer.com/permissions-api/internal/pubsub")

const (
	drainTimeout = 1 * time.Second

	eventTypeCreate = "create"
	eventTypeUpdate = "update"
	eventTypeDelete = "delete"

	fieldRelationshipSuffix = "_urn"
)

// Client represents a NATS JetStream client listening for resource lifecycle events.
type Client struct {
	logger            *zap.SugaredLogger
	nc                *nats.Conn
	js                nats.JetStreamContext
	stream            string
	prefix            string
	subscriptions     []*nats.Subscription
	resourceTypeNames []string
	qe                *query.Engine
}

// ClientOpt represents a non-config setting for a client.
type ClientOpt func(*Client)

// WithLogger sets the client logger to the given logger.
func WithLogger(l *zap.SugaredLogger) ClientOpt {
	return func(c *Client) {
		c.logger = l
	}
}

// WithResourceTypeNames sets the resource type names for the client to listen for.
func WithResourceTypeNames(typeNames []string) ClientOpt {
	return func(c *Client) {
		c.resourceTypeNames = typeNames
	}
}

// WithQueryEngine sets the query engine for the client.
func WithQueryEngine(e *query.Engine) ClientOpt {
	return func(c *Client) {
		c.qe = e
	}
}

// NewClient creates a new pubsub client.
func NewClient(cfg Config, opts ...ClientOpt) (*Client, error) {
	natsOpts := []nats.Option{
		nats.Name(cfg.Name),
		nats.UserCredentials(cfg.Credentials),
		nats.DrainTimeout(drainTimeout),
	}

	nc, err := nats.Connect(cfg.Server, natsOpts...)
	if err != nil {
		return nil, err
	}

	js, err := nc.JetStream()
	if err != nil {
		return nil, err
	}

	c := &Client{
		nc:     nc,
		js:     js,
		stream: cfg.Stream,
		prefix: cfg.Prefix,
	}

	for _, opt := range opts {
		opt(c)
	}

	return c, nil
}

func (c *Client) ensureStream() error {
	c.logger.Debugw("checking that NATS stream exists", "stream_name", c.stream)

	_, err := c.js.StreamInfo(c.stream)
	if err == nil {
		c.logger.Debugw("stream exists, not recreating", "stream_name", c.stream)
		return nil
	}

	if !errors.Is(err, nats.ErrStreamNotFound) {
		return err
	}

	_, err = c.js.AddStream(&nats.StreamConfig{
		Name: c.stream,
		Subjects: []string{
			c.prefix + ".>",
		},
		Storage:   nats.FileStorage,
		Retention: nats.LimitsPolicy,
		Discard:   nats.DiscardNew,
	})

	return err
}

// Listen ensures a stream exists, binds to it, and listens for resource lifecycle events on that stream.
func (c *Client) Listen() error {
	// Ensure stream exists before we attempt to listen on it
	err := c.ensureStream()
	if err != nil {
		return err
	}

	// Set subscription options. We specifically want manual acks in case something goes wrong
	// persisting the event
	subOpts := []nats.SubOpt{
		nats.BindStream(c.stream),
		nats.ManualAck(),
	}

	for _, name := range c.resourceTypeNames {
		subject := fmt.Sprintf("%s.%s.>", c.prefix, name)
		queueName := fmt.Sprintf("permissions-api-worker-%s", name)

		subscription, err := c.js.QueueSubscribe(subject, queueName, c.receiveMsg, subOpts...)
		if err != nil {
			return err
		}

		c.subscriptions = append(c.subscriptions, subscription)
	}

	return nil
}

// Stop drains all subscriptions for the client.
func (c *Client) Stop() error {
	for _, sub := range c.subscriptions {
		err := sub.Drain()
		if err != nil {
			return err
		}
	}

	return nil
}

func (c *Client) termMsg(msg *nats.Msg, err error) {
	c.logger.Warnw("invalid message - will not reprocess", "error", err.Error())

	err = msg.Term()
	if err != nil {
		c.logger.Errorw("error terminating message", "error", err.Error())
	}
}

func (c *Client) newResourceFromString(urnStr string) (types.Resource, error) {
	urn, err := urnx.Parse(urnStr)
	if err != nil {
		return types.Resource{}, err
	}

	return c.qe.NewResourceFromURN(urn)
}

func (c *Client) createRelationships(ctx context.Context, msg *nats.Msg, resource types.Resource, fields map[string]string) error {
	var relationships []types.Relationship

	// Attempt to create relationships from the message fields. If this fails, reject the message
	for field, value := range fields {
		relation, found := strings.CutSuffix(field, fieldRelationshipSuffix)
		if !found {
			continue
		}

		subjResource, err := c.newResourceFromString(value)
		if err != nil {
			c.logger.Errorw("error parsing field - will not reprocess", "event_type", eventTypeCreate, "field", field, "error", err.Error())
			return msg.Term()
		}

		relationship := types.Relationship{
			Resource: resource,
			Relation: relation,
			Subject:  subjResource,
		}

		relationships = append(relationships, relationship)
	}

	// Attempt to create the relationships in SpiceDB. If this fails, nak the message for reprocessing
	_, err := c.qe.CreateRelationships(ctx, relationships)
	if err != nil {
		c.logger.Errorw("error creating relationships - will reprocess", "error", err.Error())
		return msg.Nak()
	}

	return nil
}

func (c *Client) deleteRelationships(ctx context.Context, msg *nats.Msg, resource types.Resource) error {
	_, err := c.qe.DeleteRelationships(ctx, resource)
	if err != nil {
		c.logger.Errorw("error deleting relationships - will reprocess", "error", err.Error())
		return msg.Term()
	}

	return nil
}

func (c *Client) handleCreateEvent(ctx context.Context, msg *nats.Msg, payload pubsubx.Message) error {
	// Attempt to create a valid resource from the URN string. If this fails, reject the message
	resource, err := c.newResourceFromString(payload.SubjectURN)
	if err != nil {
		c.logger.Warnw("error parsing subject URN - will not reprocess", "event_type", eventTypeCreate, "error", err.Error())
		return msg.Term()
	}

	return c.createRelationships(ctx, msg, resource, payload.SubjectFields)
}

func (c *Client) handleDeleteEvent(ctx context.Context, msg *nats.Msg, payload pubsubx.Message) error {
	// Attempt to create a valid resource from the URN string. If this fails, reject the message
	resource, err := c.newResourceFromString(payload.SubjectURN)
	if err != nil {
		c.logger.Warnw("error parsing subject URN - will not reprocess", "event_type", eventTypeDelete, "error", err.Error())
		return msg.Term()
	}

	return c.deleteRelationships(ctx, msg, resource)
}

func (c *Client) handleUpdateEvent(ctx context.Context, msg *nats.Msg, payload pubsubx.Message) error {
	// Attempt to create a valid resource from the URN string. If this fails, reject the message
	resource, err := c.newResourceFromString(payload.SubjectURN)
	if err != nil {
		c.logger.Warnw("error parsing subject URN - will not reprocess", "event_type", eventTypeUpdate, "error", err.Error())
		return msg.Term()
	}

	err = c.deleteRelationships(ctx, msg, resource)
	if err != nil {
		return err
	}

	return c.createRelationships(ctx, msg, resource, payload.SubjectFields)
}

func (c *Client) handleUnknownEvent(ctx context.Context, msg *nats.Msg, payload pubsubx.Message) error {
	c.logger.Warnw("unknown event - will not reprocess", payload.EventType)

	return msg.Term()
}

func (c *Client) receiveMsg(msg *nats.Msg) {
	ctx, span := tracer.Start(context.Background(), "pubsub.receive", trace.WithAttributes(attribute.String("pubsub.subject", msg.Subject)))
	defer span.End()

	var payload pubsubx.Message

	err := json.Unmarshal(msg.Data, &payload)
	if err != nil {
		c.termMsg(msg, err)

		return
	}

	resourceURN, err := urnx.Parse(payload.SubjectURN)
	if err != nil {
		c.termMsg(msg, err)

		return
	}

	resource, err := c.qe.NewResourceFromURN(resourceURN)
	if err != nil {
		c.termMsg(msg, err)

		return
	}

	c.logger.Infow("received message", "resource_type", resource.Type, "resource_id", resource.ID.String(), "event", payload.EventType)

	switch payload.EventType {
	case eventTypeCreate:
		err = c.handleCreateEvent(ctx, msg, payload)
	case eventTypeUpdate:
		err = c.handleUpdateEvent(ctx, msg, payload)
	case eventTypeDelete:
		err = c.handleDeleteEvent(ctx, msg, payload)
	default:
		err = c.handleUnknownEvent(ctx, msg, payload)
	}

	if err != nil {
		c.logger.Errorw("error handling message", "error", err.Error())
		return
	}

	err = msg.Ack()
	if err != nil {
		c.logger.Errorw("error acking message", "error", err.Error())
		return
	}

	c.logger.Infow("successfully handled message", "resource_type", resource.Type, "resource_id", resource.ID.String(), "event", payload.EventType)
}
