package query

import (
	"context"
	"testing"

	pb "github.com/authzed/authzed-go/proto/authzed/api/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"go.infratographer.com/permissions-api/internal/iapl"
	"go.infratographer.com/permissions-api/internal/storage"
	"go.infratographer.com/permissions-api/internal/testingx"
	"go.infratographer.com/permissions-api/internal/types"
)

func rbacv2TestPolicy() iapl.Policy {
	p := DefaultPolicyV2()

	if err := p.Validate(); err != nil {
		panic(err)
	}

	return p
}

func rbacV2CreateParentRel(parent, child types.Resource, namespace string) []*pb.RelationshipUpdate {
	return []*pb.RelationshipUpdate{
		{
			Operation: pb.RelationshipUpdate_OPERATION_TOUCH,
			Relationship: &pb.Relationship{
				Resource: resourceToSpiceDBRef(namespace, child),
				Relation: "parent",
				Subject: &pb.SubjectReference{
					Object: resourceToSpiceDBRef(namespace, parent),
				},
			},
		},
		{
			Operation: pb.RelationshipUpdate_OPERATION_TOUCH,
			Relationship: &pb.Relationship{
				Resource: resourceToSpiceDBRef(namespace, child),
				Relation: "parent",
				Subject: &pb.SubjectReference{
					Object:           resourceToSpiceDBRef(namespace, parent),
					OptionalRelation: "parent",
				},
			},
		},
		{
			Operation: pb.RelationshipUpdate_OPERATION_TOUCH,
			Relationship: &pb.Relationship{
				Resource: resourceToSpiceDBRef(namespace, parent),
				Relation: "member",
				Subject: &pb.SubjectReference{
					Object:           resourceToSpiceDBRef(namespace, child),
					OptionalRelation: "member",
				},
			},
		},
	}
}

func TestCreateRolesV2(t *testing.T) {
	namespace := "testroles"
	ctx := context.Background()
	e := testEngine(ctx, t, namespace, rbacv2TestPolicy())

	tenant, err := e.NewResourceFromIDString("tnntten-root")
	require.NoError(t, err)
	actor, err := e.NewResourceFromIDString("idntusr-actor")
	require.NoError(t, err)

	// group is not a role owner in this policy
	invalidOwner, err := e.NewResourceFromIDString("idntgrp-group")
	require.NoError(t, err)

	type input struct {
		name    string
		actions []string
		owner   types.Resource
	}

	tc := []testingx.TestCase[input, types.Role]{
		{
			Name: "InvalidActions",
			Input: input{
				name:    "role1",
				actions: []string{"action1", "action2"},
				owner:   tenant,
			},
			CheckFn: func(ctx context.Context, t *testing.T, res testingx.TestResult[types.Role]) {
				require.Error(t, res.Err)
			},
		},
		{
			Name: "InvalidOwner",
			Input: input{
				name:  "lb_viewer",
				owner: invalidOwner,
				actions: []string{
					"loadbalancer_list",
					"loadbalancer_get",
				},
			},
			CheckFn: func(ctx context.Context, t *testing.T, res testingx.TestResult[types.Role]) {
				require.Error(t, res.Err)
				assert.ErrorContains(t, res.Err, "not allowed on relation")
			},
		},
		{
			Name: "CreateSuccess",
			Input: input{
				name:  "lb_viewer",
				owner: tenant,
				actions: []string{
					"loadbalancer_list",
					"loadbalancer_get",
				},
			},
			CheckFn: func(ctx context.Context, t *testing.T, res testingx.TestResult[types.Role]) {
				require.NoError(t, res.Err)

				role := res.Success
				require.Equal(t, "lb_viewer", role.Name)
				require.Len(t, role.Actions, 2)
			},
		},
	}

	testFn := func(ctx context.Context, in input) testingx.TestResult[types.Role] {
		r, err := e.CreateRoleV2(ctx, actor, in.owner, in.name, in.actions)
		if err != nil {
			return testingx.TestResult[types.Role]{Err: err}
		}

		res, err := e.NewResourceFromID(r.ID)
		if err != nil {
			return testingx.TestResult[types.Role]{Err: err}
		}

		role, err := e.GetRoleV2(ctx, res)

		return testingx.TestResult[types.Role]{Success: role, Err: err}
	}

	testingx.RunTests(ctx, t, tc, testFn)
}

func TestGetRoleV2(t *testing.T) {
	namespace := "testroles"
	ctx := context.Background()
	e := testEngine(ctx, t, namespace, rbacv2TestPolicy())

	tenant, err := e.NewResourceFromIDString("tnntten-root")
	require.NoError(t, err)
	actor, err := e.NewResourceFromIDString("idntusr-actor")
	require.NoError(t, err)

	role, err := e.CreateRoleV2(ctx, actor, tenant, "lb_viewer", []string{"loadbalancer_list", "loadbalancer_get"})
	require.NoError(t, err)

	roleRes, err := e.NewResourceFromID(role.ID)
	require.NoError(t, err)

	missingRes, err := e.NewResourceFromIDString("permrv2-notfound")
	require.NoError(t, err)

	invalidInput, err := e.NewResourceFromIDString("idntgrp-group")
	require.NoError(t, err)

	tc := []testingx.TestCase[types.Resource, types.Role]{
		{
			Name:  "GetRoleNotFound",
			Input: missingRes,
			CheckFn: func(ctx context.Context, t *testing.T, res testingx.TestResult[types.Role]) {
				assert.ErrorIs(t, res.Err, storage.ErrNoRoleFound)
			},
		},
		{
			Name:  "GetRoleInvalidInput",
			Input: invalidInput,
			CheckFn: func(ctx context.Context, t *testing.T, res testingx.TestResult[types.Role]) {
				assert.ErrorIs(t, res.Err, ErrInvalidType)
			},
		},
		{
			Name:  "GetRoleSuccess",
			Input: roleRes,
			CheckFn: func(ctx context.Context, t *testing.T, res testingx.TestResult[types.Role]) {
				require.NoError(t, res.Err)

				resp := res.Success

				require.Equal(t, role.Name, resp.Name)
				require.Len(t, resp.Actions, len(role.Actions))
			},
		},
	}

	testFn := func(ctx context.Context, in types.Resource) testingx.TestResult[types.Role] {
		role, err := e.GetRoleV2(ctx, in)
		if err != nil {
			return testingx.TestResult[types.Role]{Err: err}
		}

		return testingx.TestResult[types.Role]{Success: role, Err: err}
	}

	testingx.RunTests(ctx, t, tc, testFn)
}

func TestListRolesV2(t *testing.T) {
	namespace := "testroles"
	ctx := context.Background()
	e := testEngine(ctx, t, namespace, rbacv2TestPolicy())

	root, err := e.NewResourceFromIDString("tnntten-root")
	require.NoError(t, err)
	child, err := e.NewResourceFromIDString("tnntten-child")
	require.NoError(t, err)
	orphan, err := e.NewResourceFromIDString("tnntten-orphan")
	require.NoError(t, err)

	_, err = e.client.WriteRelationships(ctx, &pb.WriteRelationshipsRequest{
		Updates: rbacV2CreateParentRel(root, child, namespace),
	})
	require.NoError(t, err)

	actor, err := e.NewResourceFromIDString("idntusr-actor")
	require.NoError(t, err)

	_, err = e.CreateRoleV2(ctx, actor, root, "lb_viewer", []string{"loadbalancer_list", "loadbalancer_get"})
	require.NoError(t, err)

	_, err = e.CreateRoleV2(ctx, actor, root, "lb_editor", []string{"loadbalancer_list", "loadbalancer_get", "loadbalancer_update"})
	require.NoError(t, err)

	_, err = e.CreateRoleV2(ctx, actor, child, "custom_role", []string{"loadbalancer_list"})
	require.NoError(t, err)

	invalidOwner, err := e.NewResourceFromIDString("idntgrp-group")
	require.NoError(t, err)

	tc := []testingx.TestCase[types.Resource, []types.Role]{
		{
			Name:  "InvalidOwner",
			Input: invalidOwner,
			CheckFn: func(ctx context.Context, t *testing.T, res testingx.TestResult[[]types.Role]) {
				assert.ErrorIs(t, res.Err, ErrInvalidType)
			},
		},
		{
			Name:  "ListParentRoles",
			Input: root,
			CheckFn: func(ctx context.Context, t *testing.T, res testingx.TestResult[[]types.Role]) {
				assert.NoError(t, res.Err)
				assert.Len(t, res.Success, 2)
			},
		},
		{
			Name:  "ListInheritedRoles",
			Input: child,
			CheckFn: func(ctx context.Context, t *testing.T, res testingx.TestResult[[]types.Role]) {
				assert.NoError(t, res.Err)
				assert.Len(t, res.Success, 3)
			},
		},
		{
			Name:  "ListNoRoles",
			Input: orphan,
			CheckFn: func(ctx context.Context, t *testing.T, res testingx.TestResult[[]types.Role]) {
				require.NoError(t, res.Err)
				assert.Len(t, res.Success, 0)
			},
		},
	}

	testFn := func(ctx context.Context, in types.Resource) testingx.TestResult[[]types.Role] {
		roles, err := e.ListRolesV2(ctx, in)
		if err != nil {
			return testingx.TestResult[[]types.Role]{Err: err}
		}

		return testingx.TestResult[[]types.Role]{Success: roles, Err: err}
	}

	testingx.RunTests(ctx, t, tc, testFn)
}

func TestUpdateRolesV2(t *testing.T) {
	namespace := "testroles"
	ctx := context.Background()
	e := testEngine(ctx, t, namespace, rbacv2TestPolicy())

	tenant, err := e.NewResourceFromIDString("tnntten-root")
	require.NoError(t, err)
	actor, err := e.NewResourceFromIDString("idntusr-actor")
	require.NoError(t, err)

	role, err := e.CreateRoleV2(ctx, actor, tenant, "lb_viewer", []string{"loadbalancer_list", "loadbalancer_get"})
	require.NoError(t, err)

	roleRes, err := e.NewResourceFromID(role.ID)
	require.NoError(t, err)

	notfoundRes, err := e.NewResourceFromIDString("permrv2-notfound")
	require.NoError(t, err)

	type input struct {
		name    string
		actions []string
		role    types.Resource
	}

	tc := []testingx.TestCase[input, types.Role]{
		{
			Name: "UpdateRoleNotFound",
			Input: input{
				name:    "lb_viewer",
				actions: []string{"loadbalancer_list", "loadbalancer_get"},
				role:    notfoundRes,
			},
			CheckFn: func(ctx context.Context, t *testing.T, res testingx.TestResult[types.Role]) {
				assert.ErrorIs(t, res.Err, storage.ErrNoRoleFound)
			},
			Sync: true,
		},
		{
			Name: "UpdateRoleInvalidInput",
			Input: input{
				name:    "lb_viewer",
				actions: []string{"loadbalancer_list", "loadbalancer_get"},
				role:    actor,
			},
			CheckFn: func(ctx context.Context, t *testing.T, res testingx.TestResult[types.Role]) {
				require.Error(t, res.Err)
			},
			Sync: true,
		},
		{
			Name: "UpdateRoleActionNotFound",
			Input: input{
				name:    "lb_viewer",
				actions: []string{"notfound"},
				role:    roleRes,
			},
			CheckFn: func(ctx context.Context, t *testing.T, res testingx.TestResult[types.Role]) {
				assert.Equal(t, status.Code(res.Err), codes.FailedPrecondition)
				assert.ErrorContains(t, res.Err, "not found")
			},
			Sync: true,
		},
		{
			Name: "UpdateNoChange",
			Input: input{
				actions: []string{"loadbalancer_list", "loadbalancer_get"},
				role:    roleRes,
			},
			CheckFn: func(ctx context.Context, t *testing.T, res testingx.TestResult[types.Role]) {
				assert.NoError(t, res.Err)
				assert.Equal(t, role.Name, res.Success.Name)
				assert.Len(t, res.Success.Actions, len(role.Actions))
			},
			Sync: true,
		},
		{
			Name: "UpdateSuccess",
			Input: input{
				name:    "new_name",
				actions: []string{"loadbalancer_get", "loadbalancer_update"},
				role:    roleRes,
			},
			CheckFn: func(ctx context.Context, t *testing.T, res testingx.TestResult[types.Role]) {
				require.NoError(t, res.Err)

				assert.Equal(t, "new_name", res.Success.Name)
				assert.Len(t, res.Success.Actions, 2)
				assert.Contains(t, res.Success.Actions, "loadbalancer_update")
				assert.Contains(t, res.Success.Actions, "loadbalancer_get")
			},
			Sync: true,
		},
	}

	testFn := func(ctx context.Context, in input) testingx.TestResult[types.Role] {
		if _, err := e.UpdateRoleV2(ctx, actor, in.role, in.name, in.actions); err != nil {
			return testingx.TestResult[types.Role]{Err: err}
		}

		role, err := e.GetRoleV2(ctx, in.role)

		return testingx.TestResult[types.Role]{Success: role, Err: err}
	}

	testingx.RunTests(ctx, t, tc, testFn)
}

func TestDeleteRolesV2(t *testing.T) {
	namespace := "testroles"
	ctx := context.Background()
	e := testEngine(ctx, t, namespace, rbacv2TestPolicy())

	root, err := e.NewResourceFromIDString("tnntten-root")
	require.NoError(t, err)
	child, err := e.NewResourceFromIDString("tnntten-child")
	require.NoError(t, err)
	theotherchild, err := e.NewResourceFromIDString("tnntten-theotherchild")
	require.NoError(t, err)
	subj, err := e.NewResourceFromIDString("idntusr-subj")
	require.NoError(t, err)
	actor, err := e.NewResourceFromIDString("idntusr-actor")
	require.NoError(t, err)

	role, err := e.CreateRoleV2(ctx, subj, root, "lb_viewer", []string{"loadbalancer_list", "loadbalancer_get"})
	require.NoError(t, err)

	roleRes, err := e.NewResourceFromID(role.ID)
	require.NoError(t, err)

	notfoundRes, err := e.NewResourceFromIDString("permrv2-notfound")
	require.NoError(t, err)

	_, err = e.client.WriteRelationships(ctx, &pb.WriteRelationshipsRequest{
		Updates: rbacV2CreateParentRel(root, child, namespace),
	})
	require.NoError(t, err)

	_, err = e.client.WriteRelationships(ctx, &pb.WriteRelationshipsRequest{
		Updates: rbacV2CreateParentRel(root, theotherchild, namespace),
	})
	require.NoError(t, err)

	// these bindings are expected to be deleted after the role is deleted
	rbRoot, err := e.CreateRoleBinding(ctx, actor, root, roleRes, []types.RoleBindingSubject{{SubjectResource: subj}})
	require.NoError(t, err)

	rbChild, err := e.CreateRoleBinding(ctx, actor, child, roleRes, []types.RoleBindingSubject{{SubjectResource: subj}})
	require.NoError(t, err)

	rbTheOtherChild, err := e.CreateRoleBinding(ctx, actor, theotherchild, roleRes, []types.RoleBindingSubject{{SubjectResource: subj}})
	require.NoError(t, err)

	rb, err := e.ListRoleBindings(ctx, root, &roleRes)
	require.NoError(t, err)
	require.Len(t, rb, 1)

	rb, err = e.ListRoleBindings(ctx, child, &roleRes)
	require.NoError(t, err)
	require.Len(t, rb, 1)

	rb, err = e.ListRoleBindings(ctx, theotherchild, &roleRes)
	require.NoError(t, err)
	require.Len(t, rb, 1)

	tc := []testingx.TestCase[types.Resource, types.Role]{
		{
			Name:  "DeleteRoleNotFound",
			Input: notfoundRes,
			CheckFn: func(ctx context.Context, t *testing.T, res testingx.TestResult[types.Role]) {
				assert.ErrorIs(t, res.Err, storage.ErrNoRoleFound)
			},
			Sync: true,
		},
		{
			Name:  "DeleteRoleInvalidInput",
			Input: subj,
			CheckFn: func(ctx context.Context, t *testing.T, res testingx.TestResult[types.Role]) {
				assert.Error(t, res.Err)
			},
		},
		{
			Name:  "DeleteRoleWithExistingBindings",
			Input: roleRes,
			CheckFn: func(ctx context.Context, t *testing.T, res testingx.TestResult[types.Role]) {
				assert.ErrorIs(t, res.Err, ErrDeleteRoleInUse)
			},
			Sync: true,
		},
		{
			Name:  "DeleteRoleSuccess",
			Input: roleRes,
			SetupFn: func(ctx context.Context, t *testing.T) context.Context {
				var (
					err error
					rb  types.Resource
				)

				// delete the role bindings first
				rb, err = e.NewResourceFromID(rbRoot.ID)
				require.NoError(t, err)
				err = e.DeleteRoleBinding(ctx, rb)
				require.NoError(t, err)

				rb, err = e.NewResourceFromID(rbChild.ID)
				require.NoError(t, err)
				err = e.DeleteRoleBinding(ctx, rb)
				assert.NoError(t, err)

				rb, err = e.NewResourceFromID(rbTheOtherChild.ID)
				require.NoError(t, err)
				err = e.DeleteRoleBinding(ctx, rb)
				assert.NoError(t, err)

				return ctx
			},
			CheckFn: func(ctx context.Context, t *testing.T, res testingx.TestResult[types.Role]) {
				assert.NoError(t, res.Err)

				_, err := e.GetRoleV2(ctx, roleRes)
				assert.ErrorIs(t, err, storage.ErrNoRoleFound)
			},
			Sync: true,
		},
	}

	testFn := func(ctx context.Context, in types.Resource) testingx.TestResult[types.Role] {
		err := e.DeleteRoleV2(ctx, in)
		return testingx.TestResult[types.Role]{Err: err}
	}

	testingx.RunTests(ctx, t, tc, testFn)
}
