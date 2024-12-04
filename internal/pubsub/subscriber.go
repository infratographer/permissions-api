package pubsub

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"go.infratographer.com/x/events"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/multierr"
	"go.uber.org/zap"

	"go.infratographer.com/permissions-api/internal/query"
	"go.infratographer.com/permissions-api/internal/types"
)

const nakDelay = 10 * time.Second

var (
	tracer = otel.Tracer("go.infratographer.com/permissions-api/internal/pubsub")

	// ErrUnknownResourceType is returned when the corresponding resource type is not found for a resource id.
	ErrUnknownResourceType = errors.New("unknown resource type")
)

// Subscriber is the subscriber client
type Subscriber struct {
	ctx            context.Context
	changeChannels []<-chan events.Request[events.AuthRelationshipRequest, events.AuthRelationshipResponse]
	logger         *zap.SugaredLogger
	subscriber     events.AuthRelationshipSubscriber
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
func NewSubscriber(ctx context.Context, subscriber events.AuthRelationshipSubscriber, engine query.Engine, opts ...SubscriberOption) (*Subscriber, error) {
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
	msgChan, err := s.subscriber.SubscribeAuthRelationshipRequests(s.ctx, topic)
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
func (s Subscriber) listen(messages <-chan events.Request[events.AuthRelationshipRequest, events.AuthRelationshipResponse], wg *sync.WaitGroup) {
	defer wg.Done()

	for msg := range messages {
		elogger := s.logger.With(
			"event.message.topic", msg.Topic(),
			"event.message.action", msg.Message().Action,
			"event.message.object.id", msg.Message().ObjectID.String(),
			"event.message.relations", len(msg.Message().Relations),
		)

		if err := s.processEvent(msg); err != nil {
			elogger.Errorw("failed to process msg", "error", err)

			if nakErr := msg.Nak(nakDelay); nakErr != nil {
				elogger.Warnw("error occurred while naking", "error", nakErr)
			}
		} else if ackErr := msg.Ack(); ackErr != nil {
			elogger.Errorw("error occurred while acking", "error", ackErr)
		}
	}
}

// processEvent event message handler
func (s *Subscriber) processEvent(msg events.Request[events.AuthRelationshipRequest, events.AuthRelationshipResponse]) error {
	elogger := s.logger.With(
		"event.message.topic", msg.Topic(),
		"event.message.action", msg.Message().Action,
		"event.message.object.id", msg.Message().ObjectID.String(),
		"event.message.relations", len(msg.Message().Relations),
	)

	if msg.Error() != nil {
		elogger.Errorw("message contains error:", "error", msg.Error())

		return msg.Error()
	}

	request := msg.Message()

	ctx := request.GetTraceContext(context.Background())

	ctx, span := tracer.Start(ctx, "pubsub.receive", trace.WithAttributes(attribute.String("pubsub.subject", request.ObjectID.String())))

	defer span.End()

	elogger.Debugw("received message")

	var err error

	switch request.Action {
	case events.WriteAuthRelationshipAction:
		err = s.handleCreateEvent(ctx, msg)
	case events.DeleteAuthRelationshipAction:
		err = s.handleDeleteEvent(ctx, msg)
	default:
		elogger.Warnw("ignoring msg, not a write or delete action")
	}

	if err != nil {
		return err
	}

	return nil
}

func (s *Subscriber) createRelationships(ctx context.Context, relationships []types.Relationship) error {
	// Attempt to create the relationships in SpiceDB.
	if err := s.qe.CreateRelationships(ctx, relationships); err != nil {
		return fmt.Errorf("%w: error creating relationships", err)
	}

	return nil
}

func (s *Subscriber) deleteRelationships(ctx context.Context, relationships []types.Relationship) error {
	if err := s.qe.DeleteRelationships(ctx, relationships...); err != nil {
		return err
	}

	return nil
}

func (s *Subscriber) handleCreateEvent(ctx context.Context, msg events.Request[events.AuthRelationshipRequest, events.AuthRelationshipResponse]) error {
	span := trace.SpanFromContext(ctx)

	elogger := s.logger.With(
		"event.message.topic", msg.Topic(),
		"event.message.action", msg.Message().Action,
		"event.message.object.id", msg.Message().ObjectID.String(),
		"event.message.relations", len(msg.Message().Relations),
	)

	var errs []error

	if err := msg.Message().Validate(); err != nil {
		errs = multierr.Errors(err)
	}

	resource, err := s.qe.NewResourceFromID(msg.Message().ObjectID)
	if err != nil {
		span.RecordError(err)

		elogger.Warnw("error parsing resource ID", "error", err.Error())

		return respondRequest(ctx, elogger, msg, err)
	}

	rType := s.qe.GetResourceType(resource.Type)
	if rType == nil {
		err := fmt.Errorf("%w: resource: %s", ErrUnknownResourceType, resource.Type)

		span.RecordError(err)

		elogger.Warnw("error finding resource type", "error", err)

		return respondRequest(ctx, elogger, msg, err)
	}

	relationships := make([]types.Relationship, len(msg.Message().Relations))

	for i, relation := range msg.Message().Relations {
		subject, err := s.qe.NewResourceFromID(relation.SubjectID)
		if err != nil {
			verr := fmt.Errorf("error parsing subject ID: '%s': %w", relation.SubjectID, err)

			span.RecordError(verr)

			elogger.Warnw("error parsing subject ID", "error", verr.Error())

			errs = append(errs, fmt.Errorf("%w: relation %d", err, i))

			continue
		}

		sType := s.qe.GetResourceType(subject.Type)
		if sType == nil {
			err := fmt.Errorf("%w: relation %d subject: %s", ErrUnknownResourceType, i, subject.Type)

			span.RecordError(err)

			elogger.Warnw("error finding subject resource type", "error", err.Error())

			errs = append(errs, err)

			continue
		}

		relationships[i] = types.Relationship{
			Resource: resource,
			Relation: relation.Relation,
			Subject:  subject,
		}
	}

	if len(errs) != 0 {
		return respondRequest(ctx, elogger, msg, errs...)
	}

	err = s.createRelationships(ctx, relationships)

	if err != nil {
		span.RecordError(err)

		if !errors.Is(err, query.ErrInvalidRelationship) {
			span.SetStatus(codes.Error, err.Error())
		}
	}

	return respondRequest(ctx, elogger, msg, err)
}

func (s *Subscriber) handleDeleteEvent(ctx context.Context, msg events.Request[events.AuthRelationshipRequest, events.AuthRelationshipResponse]) error {
	span := trace.SpanFromContext(ctx)

	elogger := s.logger.With(
		"event.message.topic", msg.Topic(),
		"event.message.action", msg.Message().Action,
		"event.message.object.id", msg.Message().ObjectID.String(),
		"event.message.relations", len(msg.Message().Relations),
	)

	var errs []error

	if err := msg.Message().Validate(); err != nil {
		errs = multierr.Errors(err)
	}

	resource, err := s.qe.NewResourceFromID(msg.Message().ObjectID)
	if err != nil {
		span.RecordError(err)

		elogger.Warnw("error parsing resource ID", "error", err.Error())

		errs = append(errs, err)
	}

	rType := s.qe.GetResourceType(resource.Type)
	if rType == nil {
		err := fmt.Errorf("%w: resource: %s", ErrUnknownResourceType, resource.Type)

		span.RecordError(err)

		elogger.Warnw("error finding resource type", "error", err.Error())

		errs = append(errs, fmt.Errorf("%w: resource: %s", ErrUnknownResourceType, resource.Type))
	}

	relationships := make([]types.Relationship, len(msg.Message().Relations))

	for i, relation := range msg.Message().Relations {
		subject, err := s.qe.NewResourceFromID(relation.SubjectID)
		if err != nil {
			verr := fmt.Errorf("error parsing subject ID: '%s': %w", relation.SubjectID, err)

			span.RecordError(verr)

			elogger.Warnw("error parsing subject ID", "error", verr.Error())

			errs = append(errs, fmt.Errorf("%w: relation %d", err, i))

			continue
		}

		sType := s.qe.GetResourceType(subject.Type)
		if sType == nil {
			err := fmt.Errorf("%w: relation %d subject: %s", ErrUnknownResourceType, i, subject.Type)

			span.RecordError(err)

			elogger.Warnw("error finding subject resource type", "error", err.Error())

			errs = append(errs, fmt.Errorf("%w: relation %d subject: %s", ErrUnknownResourceType, i, subject.Type))

			continue
		}

		relationships[i] = types.Relationship{
			Resource: resource,
			Relation: relation.Relation,
			Subject:  subject,
		}
	}

	if len(errs) != 0 {
		return respondRequest(ctx, elogger, msg, errs...)
	}

	err = s.deleteRelationships(ctx, relationships)

	if err != nil {
		span.RecordError(err)

		if !errors.Is(err, query.ErrInvalidRelationship) {
			span.SetStatus(codes.Error, err.Error())
		}
	}

	return respondRequest(ctx, elogger, msg, err)
}

func respondRequest(ctx context.Context, logger *zap.SugaredLogger, msg events.Request[events.AuthRelationshipRequest, events.AuthRelationshipResponse], errors ...error) error {
	ctx, span := tracer.Start(ctx, "pubsub.respond")

	defer span.End()

	var filteredErrors []error

	for _, err := range errors {
		if err != nil {
			filteredErrors = append(filteredErrors, err)
		}
	}

	response := events.AuthRelationshipResponse{
		Errors: filteredErrors,
	}

	if len(filteredErrors) != 0 {
		err := multierr.Combine(filteredErrors...)

		logger.Errorw("error processing relationship, sending error response", "error", err)
	} else {
		logger.Debug("relationship successfully processed, sending response")
	}

	_, err := msg.Reply(ctx, response)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())

		logger.Errorw("error sending response", "error", err)

		return err
	}

	return nil
}
