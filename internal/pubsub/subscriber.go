package pubsub

import (
	"context"
	"sync"
	"time"

	"go.infratographer.com/permissions-api/internal/query"
	"go.infratographer.com/permissions-api/internal/types"
	"go.infratographer.com/x/events"
	"go.infratographer.com/x/gidx"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

const nakDelay = 10 * time.Second

var tracer = otel.Tracer("go.infratographer.com/permissions-api/internal/pubsub")

// Subscriber is the subscriber client
type Subscriber struct {
	ctx            context.Context
	changeChannels []<-chan events.Message[events.ChangeMessage]
	logger         *zap.SugaredLogger
	subscriber     events.Subscriber
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

// NewSubscriber creates a new Subscriber
func NewSubscriber(ctx context.Context, subscriber events.Subscriber, engine query.Engine, opts ...SubscriberOption) (*Subscriber, error) {
	s := &Subscriber{
		ctx:        ctx,
		logger:     zap.NewNop().Sugar(),
		qe:         engine,
		subscriber: subscriber,
	}

	for _, opt := range opts {
		opt(s)
	}

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
func (s Subscriber) listen(messages <-chan events.Message[events.ChangeMessage], wg *sync.WaitGroup) {
	defer wg.Done()

	for msg := range messages {
		elogger := s.logger.With(
			"event.message.id", msg.ID(),
			"event.message.timestamp", msg.Timestamp(),
			"event.message.deliveries", msg.Deliveries(),
		)

		if err := s.processEvent(msg); err != nil {
			elogger.Errorw("failed to process msg", "error", err)

			if nakErr := msg.Nak(nakDelay); nakErr != nil {
				elogger.Warnw("error occurred while naking", "error", nakErr)
			}
		} else if ackErr := msg.Ack(); ackErr != nil {
			elogger.Warnw("error occurred while acking", "error", ackErr)
		}
	}
}

// processEvent event message handler
func (s *Subscriber) processEvent(msg events.Message[events.ChangeMessage]) error {
	elogger := s.logger.With(
		"event.message.id", msg.ID(),
		"event.message.timestamp", msg.Timestamp(),
		"event.message.deliveries", msg.Deliveries(),
	)

	if msg.Error() != nil {
		elogger.Errorw("message contains error:", "error", msg.Error())

		return msg.Error()
	}

	changeMsg := msg.Message()

	ctx := changeMsg.GetTraceContext(context.Background())

	ctx, span := tracer.Start(ctx, "pubsub.receive", trace.WithAttributes(attribute.String("pubsub.subject", changeMsg.SubjectID.String())))

	defer span.End()

	elogger = elogger.With(
		"event.resource.id", changeMsg.SubjectID.String(),
		"event.type", changeMsg.EventType,
	)

	elogger.Debugw("received message")

	var err error

	switch events.ChangeType(changeMsg.EventType) {
	case events.CreateChangeType:
		err = s.handleCreateEvent(ctx, msg)
	case events.UpdateChangeType:
		err = s.handleUpdateEvent(ctx, msg)
	case events.DeleteChangeType:
		err = s.handleDeleteEvent(ctx, msg)
	default:
		elogger.Warnw("ignoring msg, not a create, update or delete event")
	}

	if err != nil {
		return err
	}

	return nil
}

func (s *Subscriber) createRelationships(ctx context.Context, msg events.Message[events.ChangeMessage], resource types.Resource, additionalSubjectIDs []gidx.PrefixedID) error {
	var relationships []types.Relationship

	rType := s.qe.GetResourceType(resource.Type)
	if rType == nil {
		s.logger.Warnw("no resource type found for", "resource_type", resource.Type)

		return nil
	}

	// Attempt to create relationships from the message fields. If this fails, reject the message
	for _, id := range additionalSubjectIDs {
		subjResource, err := s.qe.NewResourceFromID(id)
		if err != nil {
			s.logger.Warnw("error parsing additional subject id - will not reprocess", "event_type", events.CreateChangeType, "id", id.String(), "error", err.Error())

			continue
		}

		for _, rel := range rType.Relationships {
			var relation string

			for _, tName := range rel.Types {
				if tName == subjResource.Type {
					relation = rel.Relation

					break
				}
			}

			if relation != "" {
				relationship := types.Relationship{
					Resource: resource,
					Relation: relation,
					Subject:  subjResource,
				}

				relationships = append(relationships, relationship)
			}
		}
	}

	if len(relationships) == 0 {
		s.logger.Warnw("no relations to create for resource", "resource_type", resource.Type, "resource_id", resource.ID.String())

		return nil
	}

	// Attempt to create the relationships in SpiceDB. If this fails, nak the message for reprocessing
	_, err := s.qe.CreateRelationships(ctx, relationships)
	if err != nil {
		s.logger.Errorw("error creating relationships - will not reprocess", "error", err.Error())
	}

	return nil
}

func (s *Subscriber) deleteRelationships(ctx context.Context, msg events.Message[events.ChangeMessage], resource types.Resource) error {
	_, err := s.qe.DeleteRelationships(ctx, resource)
	if err != nil {
		s.logger.Errorw("error deleting relationships - will not reprocess", "error", err.Error())
	}

	return nil
}

func (s *Subscriber) handleCreateEvent(ctx context.Context, msg events.Message[events.ChangeMessage]) error {
	resource, err := s.qe.NewResourceFromID(msg.Message().SubjectID)
	if err != nil {
		s.logger.Warnw("error parsing subject ID - will not reprocess", "event_type", msg.Message().EventType, "error", err.Error())

		return nil
	}

	return s.createRelationships(ctx, msg, resource, msg.Message().AdditionalSubjectIDs)
}

func (s *Subscriber) handleDeleteEvent(ctx context.Context, msg events.Message[events.ChangeMessage]) error {
	resource, err := s.qe.NewResourceFromID(msg.Message().SubjectID)
	if err != nil {
		s.logger.Warnw("error parsing subject ID - will not reprocess", "event_type", msg.Message().EventType, "error", err.Error())

		return nil
	}

	return s.deleteRelationships(ctx, msg, resource)
}

func (s *Subscriber) handleUpdateEvent(ctx context.Context, msg events.Message[events.ChangeMessage]) error {
	resource, err := s.qe.NewResourceFromID(msg.Message().SubjectID)
	if err != nil {
		s.logger.Warnw("error parsing subject ID - will not reprocess", "event_type", msg.Message().EventType, "error", err.Error())

		return nil
	}

	err = s.deleteRelationships(ctx, msg, resource)
	if err != nil {
		return err
	}

	return s.createRelationships(ctx, msg, resource, msg.Message().AdditionalSubjectIDs)
}
