package api

import (
	"go.infratographer.com/x/gidx"
)

type createRoleRequest struct {
	Actions []string `json:"actions" binding:"required"`
}

type roleResponse struct {
	ID      gidx.PrefixedID `json:"id"`
	Actions []string        `json:"actions"`
}

type listRolesResponse struct {
	Data []roleResponse `json:"data"`
}

type createRelationshipItem struct {
	Relation  string `json:"relation" binding:"required"`
	SubjectID string `json:"subject_id" binding:"required"`
}

type createRelationshipsRequest struct {
	Relationships []createRelationshipItem `json:"relationships" binding:"required"`
}

type createRelationshipsResponse struct {
	Success bool `json:"success"`
}

type relationshipItem struct {
	Relation  string `json:"relation"`
	SubjectID string `json:"subject_id"`
}

type listRelationshipsResponse struct {
	Data []relationshipItem `json:"data"`
}

type createAssignmentRequest struct {
	SubjectID string `json:"subject_id" binding:"required"`
}

type deleteAssignmentRequest struct {
	SubjectID string `json:"subject_id" binding:"required"`
}

type createAssignmentResponse struct {
	Success bool `json:"success"`
}

type deleteAssignmentResponse struct {
	Success bool `json:"success"`
}

type assignmentItem struct {
	SubjectID string `json:"subject_id"`
}

type listAssignmentsResponse struct {
	Data []assignmentItem `json:"data"`
}
