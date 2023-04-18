package api

import (
	"net/http"

	"go.infratographer.com/permissions-api/internal/types"

	"github.com/labstack/echo/v4"
	"go.infratographer.com/x/urnx"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

func (r *Router) buildRelationship(resource types.Resource, item createRelationshipItem) (types.Relationship, error) {
	itemURN, err := urnx.Parse(item.SubjectURN)
	if err != nil {
		return types.Relationship{}, err
	}

	itemResource, err := r.engine.NewResourceFromURN(itemURN)
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

func (r *Router) relationshipsCreate(c echo.Context) error {
	resourceURNStr := c.Param("urn")

	ctx, span := tracer.Start(c.Request().Context(), "api.relationshipsCreate", trace.WithAttributes(attribute.String("urn", resourceURNStr)))
	defer span.End()

	resourceURN, err := urnx.Parse(resourceURNStr)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "error parsing resource URN").SetInternal(err)
	}

	var reqBody createRelationshipsRequest

	err = c.Bind(&reqBody)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "error parsing request body").SetInternal(err)
	}

	resource, err := r.engine.NewResourceFromURN(resourceURN)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "error creating relationships").SetInternal(err)
	}

	rels, err := r.buildRelationships(resource, reqBody.Relationships)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "error creating relationships").SetInternal(err)
	}

	_, err = r.engine.CreateRelationships(ctx, rels)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "error creating relationships").SetInternal(err)
	}

	resp := createRelationshipsResponse{
		Success: true,
	}

	return c.JSON(http.StatusCreated, resp)
}

func (r *Router) relationshipsList(c echo.Context) error {
	resourceURNStr := c.Param("urn")

	ctx, span := tracer.Start(c.Request().Context(), "api.relationshipsList", trace.WithAttributes(attribute.String("urn", resourceURNStr)))
	defer span.End()

	resourceURN, err := urnx.Parse(resourceURNStr)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "error parsing resource URN").SetInternal(err)
	}

	resource, err := r.engine.NewResourceFromURN(resourceURN)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "error listing relationships").SetInternal(err)
	}

	rels, err := r.engine.ListRelationships(ctx, resource, "")
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "error listing relationships").SetInternal(err)
	}

	items := make([]relationshipItem, len(rels))

	for i, rel := range rels {
		subjURN, err := r.engine.NewURNFromResource(rel.Subject)
		if err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, "error listing relationships").SetInternal(err)
		}

		item := relationshipItem{
			Relation:   rel.Relation,
			SubjectURN: subjURN.String(),
		}

		items[i] = item
	}

	out := listRelationshipsResponse{
		Data: items,
	}

	return c.JSON(http.StatusOK, out)
}
