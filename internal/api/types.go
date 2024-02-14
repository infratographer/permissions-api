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

// RoleBindings

type roleBindingResponseRole struct {
	ID   gidx.PrefixedID `json:"id"`
	Name string          `json:"name"`
}

type roleBindingResponse struct {
	ID       gidx.PrefixedID         `json:"id"`
	Role     roleBindingResponseRole `json:"role"`
	Subjects []string                `json:"subjects"`
}

type createRoleBindingRequest struct {
	RoleID   string   `json:"role_id" binding:"required"`
	Subjects []string `json:"subjects" binding:"required"`
}

type updateRoleBindingRequest struct {
	RoleID   string   `json:"role_id"`
	Subjects []string `json:"subjects"`
}

type listRoleBindingsResponse struct {
	Data []roleBindingResponse `json:"data"`
}

type deleteRoleBindingResponse struct {
	Success bool `json:"success"`
}
