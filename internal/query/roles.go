package query

import (
	"github.com/google/uuid"
	"go.infratographer.com/permissions-api/internal/types"
)

func newRoleFromTemplate(t types.RoleTemplate) types.Role {
	return types.Role{
		ID:      uuid.New(),
		Actions: t.Actions,
	}
}
