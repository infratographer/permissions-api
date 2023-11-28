package query

import (
	"go.infratographer.com/x/gidx"

	"go.infratographer.com/permissions-api/internal/types"
)

const (
	// ApplicationPrefix is the prefix for all application IDs owned by permissions-api
	ApplicationPrefix string = "perm"
	// RolePrefix is the prefix for roles
	RolePrefix string = ApplicationPrefix + "rol"
)

func newRole(name string, actions []string) types.Role {
	return types.Role{
		ID:      gidx.MustNewID(RolePrefix),
		Name:    name,
		Actions: actions,
	}
}
