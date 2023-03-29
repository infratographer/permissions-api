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

	_, span := tracer.Start(c.Request.Context(), "api.relationshipCreate", trace.WithAttributes(attribute.String("urn", resourceURNStr)))
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

	_, err = r.engine.CreateRelationships(c.Request.Context(), rels)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "error creating relationships", "error": err.Error()})
		return
	}

	resp := createRelationshipsResponse{
		Success: true,
	}

	c.JSON(http.StatusCreated, resp)
}
