package api

import "github.com/google/uuid"

type createRoleRequest struct {
	Actions []string `json:"actions" binding:"required"`
}

type createRoleResponse struct {
	ID      uuid.UUID `json:"id"`
	Actions []string  `json:"actions"`
}

type createRelationshipItem struct {
	Relation   string `json:"relation" binding:"required"`
	SubjectURN string `json:"subject_urn" binding:"required"`
}

type createRelationshipsRequest struct {
	Relationships []createRelationshipItem `json:"relationships" binding:"required"`
}

type createRelationshipsResponse struct {
	Success bool `json:"success"`
}

type createMembershipRequest struct {
	SubjectURN string `json:"subject_urn" binding:"required"`
}

type createMembershipResponse struct {
	Success bool `json:"success"`
}
