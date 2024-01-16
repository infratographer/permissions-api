package query

import (
	"context"
	"errors"

	pb "github.com/authzed/authzed-go/proto/authzed/api/v1"
	"github.com/nats-io/nats.go"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"

	"go.infratographer.com/permissions-api/internal/types"
)

const (
	consistencyMinimizeLatency = "minimize_latency"
	consistencyAtLeastAsFresh  = "at_least_as_fresh"
)

// getLatestZedToken attempts to get the latest ZedToken for the given resource ID.
func (e *engine) getLatestZedToken(ctx context.Context, resourceID string) (string, error) {
	_, span := e.tracer.Start(
		ctx,
		"getLatestZedToken",
		trace.WithAttributes(
			attribute.String(
				"permissions.resource",
				resourceID,
			),
		),
	)

	defer span.End()

	resp, err := e.kv.Get(resourceID)
	if err != nil {
		// Only record this as an error if it wasn't a not found error.
		if !errors.Is(err, nats.ErrKeyNotFound) {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
		}

		return "", err
	}

	zedToken := string(resp.Value())

	return zedToken, nil
}

// upsertZedToken updates the ZedToken at the given resource ID key with the provided ZedToken.
func (e *engine) upsertZedToken(ctx context.Context, resourceID string, zedToken string) error {
	_, span := e.tracer.Start(
		ctx,
		"upsertZedToken",
		trace.WithAttributes(
			attribute.String(
				"permissions.resource",
				resourceID,
			),
		),
	)

	defer span.End()

	zedTokenBytes := []byte(zedToken)

	// Attempt to get a ZedToken. If we found one, update it. If not, create it. If some other error
	// happened, log that and return
	resp, getErr := e.kv.Get(resourceID)

	var err error

	switch {
	// If we found a token, update it. This may fail if another client updated it before we did
	case getErr == nil:
		_, err = e.kv.Update(resourceID, zedTokenBytes, resp.Revision())
	// If we did not find a token, create it. This may fail if another client created an entry already
	case errors.Is(getErr, nats.ErrKeyNotFound):
		_, err = e.kv.Create(resourceID, zedTokenBytes)
	// If something else happened, just keep moving
	default:
	}

	// If an error happened when creating or updating the token, record it.
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())

		return err
	}

	return nil
}

// updateRelationshipZedTokens updates the NATS KV bucket for ZedTokens, setting the given ZedToken
// as the latest point in time snapshot for every resource in the given list of relationships.
func (e *engine) updateRelationshipZedTokens(ctx context.Context, rels []types.Relationship, zedToken string) {
	resourceIDMap := map[string]struct{}{}
	for _, rel := range rels {
		resourceIDMap[rel.Resource.ID.String()] = struct{}{}
		resourceIDMap[rel.Subject.ID.String()] = struct{}{}
	}

	for resourceID := range resourceIDMap {
		if err := e.upsertZedToken(ctx, resourceID, zedToken); err != nil {
			e.logger.Warnw("error upserting ZedToken", "error", err.Error(), "resource_id", resourceID)
		}
	}
}

// determineConsistency produces a consistency strategy based on whether a ZedToken exists for a
// given resource. If a ZedToken is available for the resource, at_least_as_fresh is used with the
// retrieved ZedToken. If no such token is found, or if there is an error reaching NATS, minimize_latency
// is used. This ensures that if NATS is not working or available for some reason, we can still make
// permissions checks (albeit in a degraded state).
func (e *engine) determineConsistency(ctx context.Context, resource types.Resource) (*pb.Consistency, string) {
	resourceID := resource.ID.String()

	zedToken, err := e.getLatestZedToken(ctx, resourceID)
	if err != nil {
		if !errors.Is(err, nats.ErrKeyNotFound) {
			e.logger.Warnw("error getting latest ZedToken - falling back to minimize_latency", "error", err.Error(), "resource_id", resourceID)
		}

		consistency := &pb.Consistency{
			Requirement: &pb.Consistency_MinimizeLatency{
				MinimizeLatency: true,
			},
		}

		return consistency, consistencyMinimizeLatency
	}

	consistency := &pb.Consistency{
		Requirement: &pb.Consistency_AtLeastAsFresh{
			AtLeastAsFresh: &pb.ZedToken{
				Token: zedToken,
			},
		},
	}

	return consistency, consistencyAtLeastAsFresh
}
