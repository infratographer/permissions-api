package api

import (
	"go.infratographer.com/x/gidx"
)

type createRoleRequest struct {
	Name    string   `json:"name" binding:"required"`
	Actions []string `json:"actions" binding:"required"`
}

type updateRoleRequest struct {
	Name    string   `json:"name"`
	Actions []string `json:"actions"`
}

type roleResponse struct {
	ID      gidx.PrefixedID `json:"id"`
	Name    string          `json:"name"`
	Actions []string        `json:"actions"`

	ResourceID gidx.PrefixedID `json:"resource_id,omitempty"`
	CreatedBy  gidx.PrefixedID `json:"created_by"`
	UpdatedBy  gidx.PrefixedID `json:"updated_by"`
	CreatedAt  string          `json:"created_at"`
	UpdatedAt  string          `json:"updated_at"`
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

type listRolesV2Response struct {
	Data []listRolesV2Role `json:"data"`
}

type listRolesV2Role struct {
	ID   gidx.PrefixedID `json:"id"`
	Name string          `json:"name"`
}
