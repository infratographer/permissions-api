package permissions

import (
	"context"
	"errors"

	"github.com/labstack/echo/v4"
	"go.infratographer.com/x/events"
	"go.infratographer.com/x/gidx"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/multierr"
)

var (
	// AuthRelationshipRequestHandlerCtxKey is the context key used to set the auth relationship request handler.
	AuthRelationshipRequestHandlerCtxKey = authRelationshipRequestHandlerCtxKey{}
)

type authRelationshipRequestHandlerCtxKey struct{}

func setAuthRelationshipRequestHandler(c echo.Context, requestHandler AuthRelationshipRequestHandler) {
	req := c.Request().WithContext(
		context.WithValue(
			c.Request().Context(),
			AuthRelationshipRequestHandlerCtxKey,
			requestHandler,
		),
	)

	c.SetRequest(req)
}

// AuthRelationshipRequestHandler defines the required methods to create or update an auth relationship.
type AuthRelationshipRequestHandler interface {
	CreateAuthRelationships(ctx context.Context, topic string, resourceID gidx.PrefixedID, relations ...events.AuthRelationshipRelation) error
	DeleteAuthRelationships(ctx context.Context, topic string, resourceID gidx.PrefixedID, relations ...events.AuthRelationshipRelation) error
}

func (p *Permissions) submitAuthRelationshipRequest(ctx context.Context, topic string, request events.AuthRelationshipRequest) error {
	ctx, span := tracer.Start(ctx, "permissions.submitAuthRelationshipRequest",
		trace.WithAttributes(
			attribute.String("events.topic", topic),
			attribute.String("events.object_id", request.ObjectID.String()),
			attribute.String("events.action", string(request.Action)),
		),
	)

	defer span.End()

	if err := request.Validate(); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())

		return err
	}

	// if no publisher is defined, requests are disabled.
	if p.publisher == nil {
		span.AddEvent("publish requests disabled")

		return nil
	}

	var errs []error

	resp, err := p.publisher.PublishAuthRelationshipRequest(ctx, topic, request)
	if err != nil {
		if p.ignoreNoResponders && errors.Is(err, events.ErrRequestNoResponders) {
			span.AddEvent("ignored no request responders")

			return nil
		}

		errs = append(errs, err)
	}

	if resp != nil {
		if resp.Error() != nil {
			errs = append(errs, err)
		}

		errs = append(errs, resp.Message().Errors...)
	}

	err = multierr.Combine(errs...)

	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())

		return err
	}

	return nil
}

// CreateAuthRelationships publishes a create auth relationship request, blocking until a response has been received.
func (p *Permissions) CreateAuthRelationships(ctx context.Context, topic string, resourceID gidx.PrefixedID, relations ...events.AuthRelationshipRelation) error {
	request := events.AuthRelationshipRequest{
		Action:    events.WriteAuthRelationshipAction,
		ObjectID:  resourceID,
		Relations: relations,
	}

	return p.submitAuthRelationshipRequest(ctx, topic, request)
}

// DeleteAuthRelationships publishes a delete auth relationship request, blocking until a response has been received.
func (p *Permissions) DeleteAuthRelationships(ctx context.Context, topic string, resourceID gidx.PrefixedID, relations ...events.AuthRelationshipRelation) error {
	request := events.AuthRelationshipRequest{
		Action:    events.DeleteAuthRelationshipAction,
		ObjectID:  resourceID,
		Relations: relations,
	}

	return p.submitAuthRelationshipRequest(ctx, topic, request)
}

// CreateAuthRelationships publishes a create auth relationship request, blocking until a response has been received.
func CreateAuthRelationships(ctx context.Context, topic string, resourceID gidx.PrefixedID, relations ...events.AuthRelationshipRelation) error {
	handler, ok := ctx.Value(AuthRelationshipRequestHandlerCtxKey).(AuthRelationshipRequestHandler)
	if !ok {
		return ErrPermissionsMiddlewareMissing
	}

	return handler.CreateAuthRelationships(ctx, topic, resourceID, relations...)
}

// DeleteAuthRelationships publishes a delete auth relationship request, blocking until a response has been received.
func DeleteAuthRelationships(ctx context.Context, topic string, resourceID gidx.PrefixedID, relations ...events.AuthRelationshipRelation) error {
	handler, ok := ctx.Value(AuthRelationshipRequestHandlerCtxKey).(AuthRelationshipRequestHandler)
	if !ok {
		return ErrPermissionsMiddlewareMissing
	}

	return handler.DeleteAuthRelationships(ctx, topic, resourceID, relations...)
}
