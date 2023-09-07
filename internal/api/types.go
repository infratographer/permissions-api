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

type resourceResponse struct {
	ID gidx.PrefixedID `json:"id"`
}

type deleteRoleResponse struct {
	Success bool `json:"success"`
}

type listRolesResponse struct {
	Data []roleResponse `json:"data"`
}

type relationshipItem struct {
	ResourceID string `json:"resource_id,omitempty"`
	Relation   string `json:"relation"`
	SubjectID  string `json:"subject_id,omitempty"`
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
