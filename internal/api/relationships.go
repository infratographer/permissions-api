package api

import (
	"net/http"

	"go.infratographer.com/permissions-api/internal/types"

	"github.com/gin-gonic/gin"
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

func (r *Router) relationshipsCreate(c *gin.Context) {
	resourceURNStr := c.Param("urn")

	ctx, span := tracer.Start(c.Request.Context(), "api.relationshipsCreate", trace.WithAttributes(attribute.String("urn", resourceURNStr)))
	defer span.End()

	resourceURN, err := urnx.Parse(resourceURNStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "error parsing resource URN", "error": err.Error()})
		return
	}

	var reqBody createRelationshipsRequest

	err = c.BindJSON(&reqBody)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "error parsing request body", "error": err.Error()})
		return
	}

	resource, err := r.engine.NewResourceFromURN(resourceURN)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "error creating relationships", "error": err.Error()})
		return
	}

	rels, err := r.buildRelationships(resource, reqBody.Relationships)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "error creating relationships", "error": err.Error()})
		return
	}

	_, err = r.engine.CreateRelationships(ctx, rels)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "error creating relationships", "error": err.Error()})
		return
	}

	resp := createRelationshipsResponse{
		Success: true,
	}

	c.JSON(http.StatusCreated, resp)
}

func (r *Router) relationshipsList(c *gin.Context) {
	resourceURNStr := c.Param("urn")

	ctx, span := tracer.Start(c.Request.Context(), "api.relationshipsList", trace.WithAttributes(attribute.String("urn", resourceURNStr)))
	defer span.End()

	resourceURN, err := urnx.Parse(resourceURNStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "error parsing resource URN", "error": err.Error()})
		return
	}

	resource, err := r.engine.NewResourceFromURN(resourceURN)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "error listing relationships", "error": err.Error()})
		return
	}

	rels, err := r.engine.ListRelationships(ctx, resource, "")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "error listing relationships", "error": err.Error()})
		return
	}

	items := make([]relationshipItem, len(rels))

	for i, rel := range rels {
		subjURN, err := r.engine.NewURNFromResource(rel.Subject)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"message": "error listing relationships", "error": err.Error()})
			return
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

	c.JSON(http.StatusOK, out)
}
