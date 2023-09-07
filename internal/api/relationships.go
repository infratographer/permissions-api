package api

import (
	"net/http"

	"go.infratographer.com/permissions-api/internal/types"

	"github.com/labstack/echo/v4"
	"go.infratographer.com/x/gidx"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

func (r *Router) buildRelationship(resource types.Resource, item createRelationshipItem) (types.Relationship, error) {
	itemID, err := gidx.Parse(item.SubjectID)
	if err != nil {
		return types.Relationship{}, err
	}

	itemResource, err := r.engine.NewResourceFromID(itemID)
	if err != nil {
		return types.Relationship{}, err
	}

	out := types.Relationship{
		Subject:  itemResource,
		Relation: item.Relation,
		Resource: resource,
	}

	return out, nil
}

func (r *Router) buildRelationships(subjResource types.Resource, items []createRelationshipItem) ([]types.Relationship, error) {
	out := make([]types.Relationship, len(items))

	for i, item := range items {
		rel, err := r.buildRelationship(subjResource, item)
		if err != nil {
			return nil, err
		}

		out[i] = rel
	}

	return out, nil
}

func (r *Router) relationshipListFrom(c echo.Context) error {
	resourceIDStr := c.Param("id")

	ctx, span := tracer.Start(c.Request().Context(), "api.relationshipListFrom", trace.WithAttributes(attribute.String("id", resourceIDStr)))
	defer span.End()

	resourceID, err := gidx.Parse(resourceIDStr)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "error parsing resource ID").SetInternal(err)
	}

	resource, err := r.engine.NewResourceFromID(resourceID)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "error listing relationships").SetInternal(err)
	}

	rels, err := r.engine.ListRelationshipsFrom(ctx, resource, "")
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "error listing relationships").SetInternal(err)
	}

	items := make([]relationshipItem, len(rels))

	for i, rel := range rels {
		items[i] = relationshipItem{
			Relation:  rel.Relation,
			SubjectID: rel.Subject.ID.String(),
		}
	}

	out := listRelationshipsResponse{
		Data: items,
	}

	return c.JSON(http.StatusOK, out)
}

func (r *Router) relationshipListTo(c echo.Context) error {
	resourceIDStr := c.Param("id")

	ctx, span := tracer.Start(c.Request().Context(), "api.relationshipListTo", trace.WithAttributes(attribute.String("id", resourceIDStr)))
	defer span.End()

	resourceID, err := gidx.Parse(resourceIDStr)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "error parsing resource ID").SetInternal(err)
	}

	resource, err := r.engine.NewResourceFromID(resourceID)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "error listing relationships").SetInternal(err)
	}

	rels, err := r.engine.ListRelationshipsTo(ctx, resource, "")
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "error listing relationships").SetInternal(err)
	}

	items := make([]relationshipItem, len(rels))

	for i, rel := range rels {
		items[i] = relationshipItem{
			ResourceID: rel.Resource.ID.String(),
			Relation:   rel.Relation,
		}
	}

	out := listRelationshipsResponse{
		Data: items,
	}

	return c.JSON(http.StatusOK, out)
}
