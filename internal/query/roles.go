package query

import (
	"github.com/google/uuid"
	"go.infratographer.com/permissions-api/internal/types"
)

func newRole(actions []string) types.Role {
	return types.Role{
		ID:      uuid.New(),
		Actions: actions,
	}
}
