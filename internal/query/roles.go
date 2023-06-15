package query

import (
	"go.infratographer.com/permissions-api/internal/types"
	"go.infratographer.com/x/gidx"
)

const (
	// ApplicationPrefix is the prefix for all application IDs owned by permissions-api
	ApplicationPrefix string = "perm"
	// RolePrefix is the prefix for roles
	RolePrefix string = ApplicationPrefix + "rol"
)

func newRole(actions []string) types.Role {
	return types.Role{
		ID:      gidx.MustNewID(RolePrefix),
		Actions: actions,
	}
}
