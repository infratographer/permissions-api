package query

import (
	"context"
	"time"

	pb "github.com/authzed/authzed-go/proto/authzed/api/v1"
	"github.com/hashicorp/golang-lru/v2/expirable"

	"go.infratographer.com/permissions-api/internal/types"
)

const (
	consistencyMinimizeLatency = "minimize_latency"
	consistencyAtLeastAsFresh  = "at_least_as_fresh"
)

func (e *engine) cacheZedTokens(resp *pb.WatchResponse) {
	zedToken := resp.ChangesThrough.Token

	for _, update := range resp.Updates {
		relationship := update.Relationship

		e.zedTokenCache.Add(
			relationship.Resource.ObjectId,
			zedToken,
		)

		e.zedTokenCache.Add(
			relationship.Subject.Object.ObjectId,
			zedToken,
		)
	}
}

// initZedTokenCache creates a new LRU cache that watches SpiceDB for ZedToken updates.
func (e *engine) initZedTokenCache(ctx context.Context) error {
	ttl := time.Minute

	watchClient, err := e.client.Watch(ctx, &pb.WatchRequest{})
	if err != nil {
		return err
	}

	lru := expirable.NewLRU[string, string](0, nil, ttl)

	e.zedTokenCache = lru

	go func() {
		for {
			resp, err := watchClient.Recv()
			if err != nil {
				e.logger.Errorf("error receiving updates", "error", err)

				time.Sleep(time.Second)

				continue
			}

			e.cacheZedTokens(resp)
		}
	}()

	return nil
}

// getLatestZedToken attempts to get the latest ZedToken for the given resource ID.
func (e *engine) getLatestZedToken(resourceID string) (string, bool) {
	resp, ok := e.zedTokenCache.Get(resourceID)
	if !ok {
		return "", false
	}

	zedToken := string(resp)

	return zedToken, true
}

// determineConsistency produces a consistency strategy based on whether a ZedToken exists for a
// given resource. If a ZedToken is available for the resource, at_least_as_fresh is used with the
// retrieved ZedToken. If no such token is found, minimize_latency is used. This ensures that if
// NATS is not working or available for some reason, we can still make permissions checks (albeit
// in a degraded state).
func (e *engine) determineConsistency(resource types.Resource) (*pb.Consistency, string) {
	resourceID := resource.ID.String()

	zedToken, ok := e.getLatestZedToken(resourceID)
	if !ok {
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
