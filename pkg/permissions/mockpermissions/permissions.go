// Package mockpermissions implements permissions.AuthRelationshipRequestHandler.
// Simplifying testing of relationship creation in applications.
package mockpermissions

import (
	"context"

	"github.com/stretchr/testify/mock"
	"go.infratographer.com/permissions-api/pkg/permissions"
	"go.infratographer.com/x/events"
	"go.infratographer.com/x/gidx"
)

var _ permissions.AuthRelationshipRequestHandler = (*MockPermissions)(nil)

// MockPermissions implements permissions.AuthRelationshipRequestHandler.
type MockPermissions struct {
	mock.Mock
}

// ContextWithHandler returns the context with the mock permissions handler defined.
func (p *MockPermissions) ContextWithHandler(ctx context.Context) context.Context {
	return context.WithValue(ctx, permissions.AuthRelationshipRequestHandlerCtxKey, p)
}

// CreateAuthRelationships implements permissions.AuthRelationshipRequestHandler.
func (p *MockPermissions) CreateAuthRelationships(ctx context.Context, topic string, resourceID gidx.PrefixedID, relations ...events.AuthRelationshipRelation) error {
	calledArgs := []interface{}{topic, resourceID}

	for _, rel := range relations {
		calledArgs = append(calledArgs, rel)
	}

	args := p.Called(calledArgs...)

	return args.Error(0)
}

// DeleteAuthRelationships implements permissions.AuthRelationshipRequestHandler.
func (p *MockPermissions) DeleteAuthRelationships(ctx context.Context, topic string, resourceID gidx.PrefixedID, relations ...events.AuthRelationshipRelation) error {
	calledArgs := []interface{}{topic, resourceID}

	for _, rel := range relations {
		calledArgs = append(calledArgs, rel)
	}

	args := p.Called(calledArgs...)

	return args.Error(0)
}
