package pubsub

import (
	"context"
	"sync"

	nc "github.com/nats-io/nats.go"
	"go.infratographer.com/permissions-api/internal/query"
	"go.infratographer.com/permissions-api/internal/types"
	"go.infratographer.com/x/events"
	"go.infratographer.com/x/gidx"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"

	"github.com/ThreeDotsLabs/watermill/message"
)

var (
	tracer = otel.Tracer("go.infratographer.com/permissions-api/internal/pubsub")
)

// Subscriber is the subscriber client
type Subscriber struct {
	ctx            context.Context
	changeChannels []<-chan *message.Message
	logger         *zap.SugaredLogger
	subscriber     *events.Subscriber
	subOpts        []nc.SubOpt
	qe             query.Engine
}

// SubscriberOption is a functional option for the Subscriber
type SubscriberOption func(s *Subscriber)

// WithLogger sets the logger for the Subscriber
func WithLogger(l *zap.SugaredLogger) SubscriberOption {
	return func(s *Subscriber) {
		s.logger = l
	}
}

// WithNatsSubOpts sets the logger for the Subscriber
func WithNatsSubOpts(options ...nc.SubOpt) SubscriberOption {
	return func(s *Subscriber) {
		s.subOpts = append(s.subOpts, options...)
	}
}

// NewSubscriber creates a new Subscriber
func NewSubscriber(ctx context.Context, cfg events.SubscriberConfig, engine query.Engine, opts ...SubscriberOption) (*Subscriber, error) {
	s := &Subscriber{
		ctx:    ctx,
		logger: zap.NewNop().Sugar(),
		qe:     engine,
	}

	for _, opt := range opts {
		opt(s)
	}

	sub, err := events.NewSubscriber(cfg, s.subOpts...)
	if err != nil {
		return nil, err
	}

	s.subscriber = sub

	s.logger.Debugw("subscriber configuration", "config", cfg)

	return s, nil
}

// Subscribe subscribes to a nats subject
func (s *Subscriber) Subscribe(topic string) error {
	msgChan, err := s.subscriber.SubscribeChanges(s.ctx, topic)
	if err != nil {
		return err
	}

	s.changeChannels = append(s.changeChannels, msgChan)

	return nil
}

// Listen start listening for messages on registered subjects and calls the registered message handler
func (s Subscriber) Listen() error {
	wg := &sync.WaitGroup{}

	// goroutine for each change channel
	for _, ch := range s.changeChannels {
		wg.Add(1)

		go s.listen(ch, wg)
	}

	wg.Wait()

	return nil
}

// listen listens for messages on a channel and calls the registered message handler
func (s Subscriber) listen(messages <-chan *message.Message, wg *sync.WaitGroup) {
	defer wg.Done()

	for msg := range messages {
		if err := s.processEvent(msg); err != nil {
			s.logger.Warn("Failed to process msg: ", err)

			msg.Nack()
		} else {
			msg.Ack()
		}
	}
}

// Close closes the subscriber connection and unsubscribes from all subscriptions
func (s *Subscriber) Close() error {
	return s.subscriber.Close()
}

// processEvent event message handler
func (s *Subscriber) processEvent(msg *message.Message) error {
	changeMsg, err := events.UnmarshalChangeMessage(msg.Payload)
	if err != nil {
		s.logger.Errorw("failed to process data in msg", zap.Error(err))

		return err
	}

	ctx, span := tracer.Start(context.Background(), "pubsub.receive", trace.WithAttributes(attribute.String("pubsub.subject", changeMsg.SubjectID.String())))

	defer span.End()

	resource, err := s.qe.NewResourceFromID(changeMsg.SubjectID)
	if err != nil {
		s.logger.Warnw("invalid subject id", "error", err.Error())

		msg.Ack()

		return err
	}

	s.logger.Infow("received message", "resource_type", resource.Type, "resource_id", resource.ID.String(), "event", changeMsg.EventType)

	switch events.ChangeType(changeMsg.EventType) {
	case events.CreateChangeType:
		err = s.handleCreateEvent(ctx, msg, changeMsg)
	case events.UpdateChangeType:
		err = s.handleUpdateEvent(ctx, msg, changeMsg)
	case events.DeleteChangeType:
		err = s.handleDeleteEvent(ctx, msg, changeMsg)
	default:
		s.logger.Debugw("ignoring msg, not a create, update or delete event", zap.String("event-type", changeMsg.EventType))
	}

	if err != nil {
		return err
	}

	return nil
}

func (s *Subscriber) createRelationships(ctx context.Context, msg *message.Message, resource types.Resource, additionalSubjectIDs []gidx.PrefixedID) error {
	var relationships []types.Relationship

	// Attempt to create relationships from the message fields. If this fails, reject the message
	for _, id := range additionalSubjectIDs {
		subjResource, err := s.qe.NewResourceFromID(id)
		if err != nil {
			s.logger.Warnw("error parsing additional subject id - will not reprocess", "event_type", events.CreateChangeType, "id", id.String(), "error", err.Error())

			return nil
		}

		relationship := types.Relationship{
			Resource: resource,
			Relation: subjResource.Type,
			Subject:  subjResource,
		}

		relationships = append(relationships, relationship)
	}

	// Attempt to create the relationships in SpiceDB. If this fails, nak the message for reprocessing
	_, err := s.qe.CreateRelationships(ctx, relationships)
	if err != nil {
		s.logger.Errorw("error creating relationships - will reprocess", "error", err.Error())

		return err
	}

	return nil
}

func (s *Subscriber) deleteRelationships(ctx context.Context, msg *message.Message, resource types.Resource) error {
	_, err := s.qe.DeleteRelationships(ctx, resource)
	if err != nil {
		s.logger.Errorw("error deleting relationships - will reprocess", "error", err.Error())
		return err
	}

	return nil
}

func (s *Subscriber) handleCreateEvent(ctx context.Context, msg *message.Message, changeMsg events.ChangeMessage) error {
	resource, err := s.qe.NewResourceFromID(changeMsg.SubjectID)
	if err != nil {
		s.logger.Warnw("error parsing subject ID - will not reprocess", "event_type", changeMsg.EventType, "error", err.Error())

		return nil
	}

	return s.createRelationships(ctx, msg, resource, changeMsg.AdditionalSubjectIDs)
}

func (s *Subscriber) handleDeleteEvent(ctx context.Context, msg *message.Message, changeMsg events.ChangeMessage) error {
	resource, err := s.qe.NewResourceFromID(changeMsg.SubjectID)
	if err != nil {
		s.logger.Warnw("error parsing subject ID - will not reprocess", "event_type", changeMsg.EventType, "error", err.Error())

		return nil
	}

	return s.deleteRelationships(ctx, msg, resource)
}

func (s *Subscriber) handleUpdateEvent(ctx context.Context, msg *message.Message, changeMsg events.ChangeMessage) error {
	resource, err := s.qe.NewResourceFromID(changeMsg.SubjectID)
	if err != nil {
		s.logger.Warnw("error parsing subject ID - will not reprocess", "event_type", changeMsg.EventType, "error", err.Error())

		return nil
	}

	err = s.deleteRelationships(ctx, msg, resource)
	if err != nil {
		return err
	}

	return s.createRelationships(ctx, msg, resource, changeMsg.AdditionalSubjectIDs)
}
