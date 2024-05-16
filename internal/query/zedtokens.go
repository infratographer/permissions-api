package query

import (
	"context"

	"go.infratographer.com/permissions-api/internal/types"

	pb "github.com/authzed/authzed-go/proto/authzed/api/v1"
	"go.infratographer.com/x/gidx"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

const (
	consistencyMinimizeLatency = "minimize_latency"
	consistencyAtLeastAsFresh  = "at_least_as_fresh"
)

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

	prefixedID, err := gidx.Parse(resourceID)
	if err != nil {
		return err
	}

	err = e.store.UpsertZedToken(ctx, prefixedID, zedToken)

	// If an error happened when creating or updating the token, record it.
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())

		return err
	}

	return nil
}

// updateRelationshipZedTokens updates the CRDB table for ZedTokens, setting the given ZedToken
// as the latest point in time snapshot for every resource in the given list of relationships.
//
// This function updates the table using an out of band transaction, as if it fails we do not want
// to roll back the entire outer transaction.
func (e *engine) updateRelationshipZedTokens(ctx context.Context, rels []types.Relationship, zedToken string) {
	resourceIDMap := map[string]struct{}{}
	for _, rel := range rels {
		resourceIDMap[rel.Resource.ID.String()] = struct{}{}
		resourceIDMap[rel.Subject.ID.String()] = struct{}{}
	}

	ctx, span := e.tracer.Start(
		ctx,
		"updateRelationshipZedTokens",
	)

	defer span.End()

	dbCtx, err := e.store.BeginContext(ctx)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())

		return
	}

	for resourceID := range resourceIDMap {
		if err := e.upsertZedToken(dbCtx, resourceID, zedToken); err != nil {
			e.logger.Warnw("error upserting ZedToken", "error", err.Error(), "resource_id", resourceID)
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())

			logRollbackErr(e.logger, e.store.RollbackContext(dbCtx))

			return
		}
	}

	if err = e.store.CommitContext(dbCtx); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())

		logRollbackErr(e.logger, e.store.RollbackContext(dbCtx))
	}
}

// determineConsistency produces a consistency strategy based on whether a ZedToken exists for a
// given resource. If a ZedToken is available for the resource, at_least_as_fresh is used with the
// retrieved ZedToken. If no such token is found, minimize_latency is used. This ensures that if
// NATS is not working or available for some reason, we can still make permissions checks (albeit
// in a degraded state).
func (e *engine) determineConsistency(ctx context.Context, resource types.Resource) (*pb.Consistency, string) {
	resourceID := resource.ID

	_, span := e.tracer.Start(
		ctx,
		"determineConsistency",
		trace.WithAttributes(
			attribute.Stringer(
				"permissions.resource",
				resourceID,
			),
		),
	)

	defer span.End()

	consistency := &pb.Consistency{
		Requirement: &pb.Consistency_MinimizeLatency{
			MinimizeLatency: true,
		},
	}

	consistencyName := consistencyMinimizeLatency

	zedToken, err := e.store.GetLatestZedToken(ctx, resourceID)

	switch {
	case err != nil:
		e.logger.Warnw("error getting ZedToken", "error", err.Error())
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	case zedToken != "":
		consistency = &pb.Consistency{
			Requirement: &pb.Consistency_AtLeastAsFresh{
				AtLeastAsFresh: &pb.ZedToken{
					Token: zedToken,
				},
			},
		}

		consistencyName = consistencyAtLeastAsFresh
	}

	return consistency, consistencyName
}
