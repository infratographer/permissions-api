package query

import (
	"context"
	"testing"

	pb "github.com/authzed/authzed-go/proto/authzed/api/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.infratographer.com/permissions-api/internal/iapl"
	"go.infratographer.com/permissions-api/internal/storage"
	"go.infratographer.com/permissions-api/internal/testingx"
	"go.infratographer.com/permissions-api/internal/types"
)

func TestCreateRoleBinding(t *testing.T) {
	namespace := "testroles"
	ctx := context.Background()

	doc := DefaultPolicyDocumentV2()
	doc.ResourceTypes = append(doc.ResourceTypes, iapl.ResourceType{
		Name:     "role",
		IDPrefix: "permrol",
		Relationships: []iapl.Relationship{
			{
				Relation:    "subject",
				TargetTypes: []types.TargetType{{Name: "subject"}},
			},
		},
	})

	policy := iapl.NewPolicy(doc)
	err := policy.Validate()
	require.NoError(t, err)

	e := testEngine(ctx, t, namespace, policy)

	root, err := e.NewResourceFromIDString("tnntten-root")
	require.NoError(t, err)
	child, err := e.NewResourceFromIDString("tnntten-child")
	require.NoError(t, err)
	orphan, err := e.NewResourceFromIDString("tnntten-orphan")
	require.NoError(t, err)
	subj, err := e.NewResourceFromIDString("idntusr-subj")
	require.NoError(t, err)
	actor, err := e.NewResourceFromIDString("idntusr-actor")
	require.NoError(t, err)

	role, err := e.CreateRoleV2(ctx, subj, root, "lb_viewer", []string{"loadbalancer_list", "loadbalancer_get"})
	require.NoError(t, err)

	roleRes, err := e.NewResourceFromID(role.ID)
	require.NoError(t, err)

	notfoundRole, err := e.NewResourceFromIDString("permrv2-notfound")
	require.NoError(t, err)

	v1role, err := e.NewResourceFromIDString("permrol-v1role")
	require.NoError(t, err)

	_, err = e.client.WriteRelationships(ctx, &pb.WriteRelationshipsRequest{
		Updates: rbacV2CreateParentRel(root, child, namespace),
	})
	require.NoError(t, err)

	type input struct {
		resource types.Resource
		role     types.Resource
		subjects []types.RoleBindingSubject
	}

	tc := []testingx.TestCase[input, types.RoleBinding]{
		{
			Name: "CreateRoleBindingRoleNotFound",
			Input: input{
				resource: root,
				role:     notfoundRole,
				subjects: []types.RoleBindingSubject{{SubjectResource: subj}},
			},
			CheckFn: func(ctx context.Context, t *testing.T, res testingx.TestResult[types.RoleBinding]) {
				assert.ErrorIs(t, res.Err, ErrRoleNotFound)
			},
		},
		{
			Name: "CreateRoleBindingV1Role",
			Input: input{
				resource: root,
				role:     v1role,
				subjects: []types.RoleBindingSubject{{SubjectResource: subj}},
			},
			CheckFn: func(ctx context.Context, t *testing.T, res testingx.TestResult[types.RoleBinding]) {
				assert.ErrorIs(t, res.Err, ErrRoleNotFound)
			},
		},
		{
			Name: "CreateRoleBindingChild",
			Input: input{
				resource: child,
				role:     roleRes,
				subjects: []types.RoleBindingSubject{{SubjectResource: subj}},
			},
			CheckFn: func(ctx context.Context, t *testing.T, res testingx.TestResult[types.RoleBinding]) {
				assert.NoError(t, res.Err)
				assert.Equal(t, role.ID, res.Success.RoleID)
				assert.Len(t, res.Success.SubjectIDs, 1)

				rb, err := e.ListRoleBindings(ctx, child, nil)
				assert.NoError(t, err)
				assert.Len(t, rb, 1)
			},
		},
		{
			Name: "CreateRoleBindingOrphan",
			Input: input{
				resource: orphan,
				role:     roleRes,
				subjects: []types.RoleBindingSubject{{SubjectResource: subj}},
			},
			CheckFn: func(ctx context.Context, t *testing.T, res testingx.TestResult[types.RoleBinding]) {
				assert.ErrorIs(t, res.Err, ErrRoleNotFound)
			},
		},
		{
			Name: "CreateRoleBindingWithNoSubjects",
			Input: input{
				resource: root,
				role:     roleRes,
			},
			CheckFn: func(ctx context.Context, t *testing.T, tr testingx.TestResult[types.RoleBinding]) {
				assert.ErrorIs(t, tr.Err, ErrInvalidArgument)
				assert.ErrorIs(t, tr.Err, ErrCreateRoleBindingWithNoSubjects)
			},
		},
		{
			Name: "CreateRoleBindingSuccess",
			Input: input{
				resource: root,
				role:     roleRes,
				subjects: []types.RoleBindingSubject{{SubjectResource: subj}},
			},
			CheckFn: func(ctx context.Context, t *testing.T, res testingx.TestResult[types.RoleBinding]) {
				assert.NoError(t, res.Err)

				assert.Len(t, res.Success.SubjectIDs, 1)
				assert.Equal(t, role.ID, res.Success.RoleID)
				assert.Equal(t, root.ID, res.Success.ResourceID)
				assert.Equal(t, subj.ID, res.Success.SubjectIDs[0])
				assert.Equal(t, actor.ID, res.Success.CreatedBy)

				rbs, err := e.ListRoleBindings(ctx, root, nil)
				assert.NoError(t, err)
				assert.Len(t, rbs, 1)
			},
		},
	}

	testFn := func(ctx context.Context, in input) testingx.TestResult[types.RoleBinding] {
		rb, err := e.CreateRoleBinding(ctx, actor, in.resource, in.role, in.subjects)
		return testingx.TestResult[types.RoleBinding]{Success: rb, Err: err}
	}

	testingx.RunTests(ctx, t, tc, testFn)
}

func TestListRoleBindings(t *testing.T) {
	namespace := "testroles"
	ctx := context.Background()
	e := testEngine(ctx, t, namespace, rbacv2TestPolicy())

	root, err := e.NewResourceFromIDString("tnntten-root")
	require.NoError(t, err)
	child, err := e.NewResourceFromIDString("tnntten-child")
	require.NoError(t, err)
	subj, err := e.NewResourceFromIDString("idntusr-subj")
	require.NoError(t, err)
	actor, err := e.NewResourceFromIDString("idntusr-actor")
	require.NoError(t, err)

	viewer, err := e.CreateRoleV2(ctx, subj, root, "lb_viewer", []string{"loadbalancer_list", "loadbalancer_get"})
	require.NoError(t, err)

	editor, err := e.CreateRoleV2(ctx, subj, root, "lb_editor", []string{"loadbalancer_list", "loadbalancer_get", "loadbalancer_create", "loadbalancer_update"})
	require.NoError(t, err)

	viewerRes, err := e.NewResourceFromID(viewer.ID)
	require.NoError(t, err)

	editorRes, err := e.NewResourceFromID(editor.ID)
	require.NoError(t, err)

	notfoundRole, err := e.NewResourceFromIDString("permrv2-notfound")
	require.NoError(t, err)

	_, err = e.CreateRoleBinding(ctx, actor, root, viewerRes, []types.RoleBindingSubject{{SubjectResource: subj}})
	require.NoError(t, err)

	_, err = e.CreateRoleBinding(ctx, actor, root, editorRes, []types.RoleBindingSubject{{SubjectResource: subj}})
	require.NoError(t, err)

	_, err = e.client.WriteRelationships(ctx, &pb.WriteRelationshipsRequest{
		Updates: rbacV2CreateParentRel(root, child, namespace),
	})
	require.NoError(t, err)

	type input struct {
		resource types.Resource
		role     *types.Resource
	}

	tc := []testingx.TestCase[input, []types.RoleBinding]{
		{
			Name: "ListAll",
			Input: input{
				resource: root,
			},
			CheckFn: func(ctx context.Context, t *testing.T, res testingx.TestResult[[]types.RoleBinding]) {
				assert.Len(t, res.Success, 2)
			},
		},
		{
			Name: "ListWithViewerRole",
			Input: input{
				resource: root,
				role:     &viewerRes,
			},
			CheckFn: func(ctx context.Context, t *testing.T, res testingx.TestResult[[]types.RoleBinding]) {
				assert.Len(t, res.Success, 1)
				assert.Equal(t, viewer.ID, res.Success[0].RoleID)
			},
		},
		{
			Name: "ListWithEditorRole",
			Input: input{
				resource: root,
				role:     &editorRes,
			},
			CheckFn: func(ctx context.Context, t *testing.T, res testingx.TestResult[[]types.RoleBinding]) {
				assert.Len(t, res.Success, 1)
				assert.Equal(t, editor.ID, res.Success[0].RoleID)
			},
		},
		{
			Name: "ListChildTenant",
			Input: input{
				resource: child,
			},
			CheckFn: func(ctx context.Context, t *testing.T, res testingx.TestResult[[]types.RoleBinding]) {
				assert.Len(t, res.Success, 0)
			},
		},
		{
			Name: "ListWithNonExistentRole",
			Input: input{
				resource: root,
				role:     &notfoundRole,
			},
			CheckFn: func(ctx context.Context, t *testing.T, res testingx.TestResult[[]types.RoleBinding]) {
				assert.Len(t, res.Success, 0)
			},
		},
	}

	testFn := func(ctx context.Context, in input) testingx.TestResult[[]types.RoleBinding] {
		rb, err := e.ListRoleBindings(ctx, in.resource, in.role)
		return testingx.TestResult[[]types.RoleBinding]{Success: rb, Err: err}
	}

	testingx.RunTests(ctx, t, tc, testFn)
}

func TestGetRoleBinding(t *testing.T) {
	namespace := "testroles"
	ctx := context.Background()
	e := testEngine(ctx, t, namespace, rbacv2TestPolicy())

	root, err := e.NewResourceFromIDString("tnntten-root")
	require.NoError(t, err)
	subj, err := e.NewResourceFromIDString("idntusr-subj")
	require.NoError(t, err)
	actor, err := e.NewResourceFromIDString("idntusr-actor")
	require.NoError(t, err)

	viewer, err := e.CreateRoleV2(ctx, subj, root, "lb_viewer", []string{"loadbalancer_list", "loadbalancer_get"})
	require.NoError(t, err)

	viewerRes, err := e.NewResourceFromID(viewer.ID)
	require.NoError(t, err)

	notfoundRB, err := e.NewResourceFromIDString("permrbn-notfound")
	require.NoError(t, err)

	rb, err := e.CreateRoleBinding(ctx, actor, root, viewerRes, []types.RoleBindingSubject{{SubjectResource: subj}})
	require.NoError(t, err)

	rbRes, err := e.NewResourceFromID(rb.ID)
	require.NoError(t, err)

	tc := []testingx.TestCase[types.Resource, types.RoleBinding]{
		{
			Name:  "GetRoleBindingSuccess",
			Input: rbRes,
			CheckFn: func(ctx context.Context, t *testing.T, res testingx.TestResult[types.RoleBinding]) {
				assert.NoError(t, res.Err)
				assert.Equal(t, viewer.ID, res.Success.RoleID)
				assert.Len(t, res.Success.SubjectIDs, 1)
				assert.Equal(t, subj.ID, res.Success.SubjectIDs[0])
				assert.Equal(t, actor.ID, res.Success.CreatedBy)
				assert.Equal(t, root.ID, res.Success.ResourceID)
			},
		},
		{
			Name:  "GetRoleBindingNotFound",
			Input: notfoundRB,
			CheckFn: func(ctx context.Context, t *testing.T, res testingx.TestResult[types.RoleBinding]) {
				assert.ErrorContains(t, res.Err, ErrRoleBindingNotFound.Error())
			},
		},
	}

	testFn := func(ctx context.Context, in types.Resource) testingx.TestResult[types.RoleBinding] {
		rb, err := e.GetRoleBinding(ctx, in)
		return testingx.TestResult[types.RoleBinding]{Success: rb, Err: err}
	}

	testingx.RunTests(ctx, t, tc, testFn)
}

func TestUpdateRoleBinding(t *testing.T) {
	namespace := "testroles"
	ctx := context.Background()
	e := testEngine(ctx, t, namespace, rbacv2TestPolicy())

	root, err := e.NewResourceFromIDString("tnntten-root")
	require.NoError(t, err)
	subj, err := e.NewResourceFromIDString("idntusr-subj")
	require.NoError(t, err)
	actor, err := e.NewResourceFromIDString("idntusr-actor")
	require.NoError(t, err)

	viewer, err := e.CreateRoleV2(ctx, subj, root, "lb_viewer", []string{"loadbalancer_list", "loadbalancer_get"})
	require.NoError(t, err)
	viewerRes, err := e.NewResourceFromID(viewer.ID)
	require.NoError(t, err)

	rb, err := e.CreateRoleBinding(ctx, subj, root, viewerRes, []types.RoleBindingSubject{{SubjectResource: subj}})
	require.NoError(t, err)
	rbRes, err := e.NewResourceFromID(rb.ID)
	require.NoError(t, err)
	notfoundRB, err := e.NewResourceFromIDString("permrbn-notfound")
	require.NoError(t, err)

	user1, err := e.NewResourceFromIDString("idntusr-user1")
	require.NoError(t, err)
	group1, err := e.NewResourceFromIDString("idntgrp-group1")
	require.NoError(t, err)
	invalidsubj, err := e.NewResourceFromIDString("loadbal-lb")
	require.NoError(t, err)

	type input struct {
		rb   types.Resource
		subj []types.RoleBindingSubject
	}

	tc := []testingx.TestCase[input, types.RoleBinding]{
		{
			Name: "UpdateRoleBindingNotFound",
			Input: input{
				rb:   notfoundRB,
				subj: []types.RoleBindingSubject{{SubjectResource: subj}},
			},
			CheckFn: func(ctx context.Context, t *testing.T, res testingx.TestResult[types.RoleBinding]) {
				assert.ErrorContains(t, res.Err, ErrRoleBindingNotFound.Error())
			},
			Sync: true,
		},
		{
			Name: "UpdateRoleBindingInvalidSubject",
			Input: input{
				rb:   rbRes,
				subj: []types.RoleBindingSubject{{SubjectResource: invalidsubj}},
			},
			CheckFn: func(ctx context.Context, t *testing.T, res testingx.TestResult[types.RoleBinding]) {
				assert.ErrorContains(t, res.Err, ErrInvalidArgument.Error())
			},
			Sync: true,
		},
		{
			Name: "UpdateRoleBindingSuccess",
			Input: input{
				rb:   rbRes,
				subj: []types.RoleBindingSubject{{SubjectResource: user1}, {SubjectResource: group1}},
			},
			CheckFn: func(ctx context.Context, t *testing.T, res testingx.TestResult[types.RoleBinding]) {
				assert.NoError(t, res.Err)

				assert.Len(t, res.Success.SubjectIDs, 2)
				assert.Contains(t, res.Success.SubjectIDs, user1.ID)
				assert.Contains(t, res.Success.SubjectIDs, group1.ID)
				assert.NotContains(t, res.Success.SubjectIDs, subj.ID)

				assert.Equal(t, actor.ID, res.Success.UpdatedBy)
				assert.Equal(t, root.ID, res.Success.ResourceID)
				assert.Equal(t, subj.ID, res.Success.CreatedBy)
			},
			Sync: true,
		},
	}

	testFn := func(ctx context.Context, in input) testingx.TestResult[types.RoleBinding] {
		rb, err := e.UpdateRoleBinding(ctx, actor, in.rb, in.subj)
		return testingx.TestResult[types.RoleBinding]{Success: rb, Err: err}
	}

	testingx.RunTests(ctx, t, tc, testFn)
}

func TestDeleteRoleBinding(t *testing.T) {
	namespace := "testroles"
	ctx := context.Background()
	e := testEngine(ctx, t, namespace, rbacv2TestPolicy())

	root, err := e.NewResourceFromIDString("tnntten-root")
	require.NoError(t, err)
	actor, err := e.NewResourceFromIDString("idntusr-actor")
	require.NoError(t, err)

	viewer, err := e.CreateRoleV2(ctx, actor, root, "lb_viewer", []string{"loadbalancer_list", "loadbalancer_get"})
	require.NoError(t, err)
	viewerRes, err := e.NewResourceFromID(viewer.ID)
	require.NoError(t, err)

	rb, err := e.CreateRoleBinding(ctx, actor, root, viewerRes, []types.RoleBindingSubject{{SubjectResource: actor}})
	require.NoError(t, err)
	rbRes, err := e.NewResourceFromID(rb.ID)
	require.NoError(t, err)

	notfoundRB, err := e.NewResourceFromIDString("permrbn-notfound")
	require.NoError(t, err)

	tc := []testingx.TestCase[types.Resource, types.RoleBinding]{
		{
			Name:  "DeleteRoleBindingNotFound",
			Input: notfoundRB,
			CheckFn: func(ctx context.Context, t *testing.T, res testingx.TestResult[types.RoleBinding]) {
				assert.ErrorIs(t, res.Err, storage.ErrRoleBindingNotFound)

				rb, err := e.ListRoleBindings(ctx, root, nil)
				assert.NoError(t, err)
				assert.Len(t, rb, 1)
			},
			Sync: true,
		},
		{
			Name:  "DeleteRoleBindingSuccess",
			Input: rbRes,
			CheckFn: func(ctx context.Context, t *testing.T, res testingx.TestResult[types.RoleBinding]) {
				assert.NoError(t, res.Err)

				rb, err := e.ListRoleBindings(ctx, root, nil)
				assert.NoError(t, err)
				assert.Len(t, rb, 0)
			},
			Sync: true,
		},
	}

	testFn := func(ctx context.Context, in types.Resource) testingx.TestResult[types.RoleBinding] {
		err := e.DeleteRoleBinding(ctx, in)
		return testingx.TestResult[types.RoleBinding]{Err: err}
	}

	testingx.RunTests(ctx, t, tc, testFn)
}

func TestPermissions(t *testing.T) {
	namespace := "testroles"
	ctx := context.Background()
	e := testEngine(ctx, t, namespace, rbacv2TestPolicy())

	root, err := e.NewResourceFromIDString("tnntten-root")
	require.NoError(t, err)
	child, err := e.NewResourceFromIDString("tnntten-child")
	require.NoError(t, err)
	actor, err := e.NewResourceFromIDString("idntusr-actor")
	require.NoError(t, err)

	// create child tenant relationships
	_, err = e.client.WriteRelationships(ctx, &pb.WriteRelationshipsRequest{
		Updates: rbacV2CreateParentRel(root, child, namespace),
	})
	require.NoError(t, err)

	// role
	viewer, err := e.CreateRoleV2(ctx, actor, root, "lb_viewer", []string{"loadbalancer_list", "loadbalancer_get"})
	require.NoError(t, err)
	viewerRes, err := e.NewResourceFromID(viewer.ID)
	require.NoError(t, err)

	// subjects
	user1, err := e.NewResourceFromIDString("idntusr-user1")
	require.NoError(t, err)
	user2, err := e.NewResourceFromIDString("idntusr-user2")
	require.NoError(t, err)
	group1, err := e.NewResourceFromIDString("idntgrp-group1")
	require.NoError(t, err)

	// rolebinding
	var rb types.RoleBinding

	err = e.CreateRelationships(ctx, []types.Relationship{{
		Resource: group1,
		Relation: "member",
		Subject:  user2,
	}})
	require.NoError(t, err)

	_, err = e.client.WriteRelationships(ctx, &pb.WriteRelationshipsRequest{
		Updates: rbacV2CreateParentRel(root, group1, namespace),
	})
	require.NoError(t, err)

	// resources
	lb1, err := e.NewResourceFromIDString("loadbal-lb1")
	require.NoError(t, err)

	err = e.CreateRelationships(ctx, []types.Relationship{{
		Resource: lb1,
		Relation: "owner",
		Subject:  child,
	}})
	require.NoError(t, err)

	fullconsistency := &pb.Consistency{Requirement: &pb.Consistency_FullyConsistent{FullyConsistent: true}}

	tc := []testingx.TestCase[any, any]{
		{
			Name: "PermissionsOnResource",
			SetupFn: func(ctx context.Context, t *testing.T) context.Context {
				err := e.checkPermission(ctx, &pb.CheckPermissionRequest{
					Consistency: fullconsistency,
					Resource:    resourceToSpiceDBRef(namespace, lb1),
					Permission:  "loadbalancer_get",
					Subject:     &pb.SubjectReference{Object: resourceToSpiceDBRef(namespace, user1)},
				})
				require.Error(t, err)

				_, err = e.CreateRoleBinding(ctx, user1, lb1, viewerRes, []types.RoleBindingSubject{{SubjectResource: user1}})
				require.NoError(t, err)

				return ctx
			},
			CheckFn: func(ctx context.Context, t *testing.T, _ testingx.TestResult[any]) {
				err := e.checkPermission(ctx, &pb.CheckPermissionRequest{
					Consistency: fullconsistency,
					Resource:    resourceToSpiceDBRef(namespace, lb1),
					Permission:  "loadbalancer_get",
					Subject:     &pb.SubjectReference{Object: resourceToSpiceDBRef(namespace, user1)},
				})
				assert.NoError(t, err)
			},
			CleanupFn: func(ctx context.Context) {
				rbs, _ := e.ListRoleBindings(ctx, lb1, nil)
				for _, rb := range rbs {
					rbRes, _ := e.NewResourceFromID(rb.ID)
					_ = e.DeleteRoleBinding(ctx, rbRes)
				}
			},
			Sync: true,
		},
		{
			Name: "PermissionsOnOwner",
			SetupFn: func(ctx context.Context, t *testing.T) context.Context {
				err := e.checkPermission(ctx, &pb.CheckPermissionRequest{
					Consistency: fullconsistency,
					Resource:    resourceToSpiceDBRef(namespace, lb1),
					Permission:  "loadbalancer_get",
					Subject:     &pb.SubjectReference{Object: resourceToSpiceDBRef(namespace, user1)},
				})
				require.Error(t, err)

				_, err = e.CreateRoleBinding(ctx, user1, child, viewerRes, []types.RoleBindingSubject{{SubjectResource: user1}})
				require.NoError(t, err)

				return ctx
			},
			CheckFn: func(ctx context.Context, t *testing.T, _ testingx.TestResult[any]) {
				err := e.checkPermission(ctx, &pb.CheckPermissionRequest{
					Consistency: fullconsistency,
					Resource:    resourceToSpiceDBRef(namespace, lb1),
					Permission:  "loadbalancer_get",
					Subject:     &pb.SubjectReference{Object: resourceToSpiceDBRef(namespace, user1)},
				})
				assert.NoError(t, err)
			},
			CleanupFn: func(ctx context.Context) {
				rbs, _ := e.ListRoleBindings(ctx, child, nil)
				for _, rb := range rbs {
					rbRes, _ := e.NewResourceFromID(rb.ID)
					_ = e.DeleteRoleBinding(ctx, rbRes)
				}
			},
			Sync: true,
		},
		{
			Name: "PermissionsOnOwnerParent",
			SetupFn: func(ctx context.Context, t *testing.T) context.Context {
				err := e.checkPermission(ctx, &pb.CheckPermissionRequest{
					Consistency: fullconsistency,
					Resource:    resourceToSpiceDBRef(namespace, lb1),
					Permission:  "loadbalancer_get",
					Subject:     &pb.SubjectReference{Object: resourceToSpiceDBRef(namespace, user1)},
				})
				require.Error(t, err)

				_, err = e.CreateRoleBinding(ctx, user1, root, viewerRes, []types.RoleBindingSubject{{SubjectResource: user1}})
				require.NoError(t, err)

				return ctx
			},
			CheckFn: func(ctx context.Context, t *testing.T, _ testingx.TestResult[any]) {
				err := e.checkPermission(ctx, &pb.CheckPermissionRequest{
					Consistency: fullconsistency,
					Resource:    resourceToSpiceDBRef(namespace, lb1),
					Permission:  "loadbalancer_get",
					Subject:     &pb.SubjectReference{Object: resourceToSpiceDBRef(namespace, user1)},
				})
				assert.NoError(t, err)
			},
			CleanupFn: func(ctx context.Context) {
				rbs, _ := e.ListRoleBindings(ctx, root, nil)
				for _, rb := range rbs {
					rbRes, _ := e.NewResourceFromID(rb.ID)
					_ = e.DeleteRoleBinding(ctx, rbRes)
				}
			},
			Sync: true,
		},
		{
			Name: "PermissionsOnGroups",
			SetupFn: func(ctx context.Context, t *testing.T) context.Context {
				err := e.checkPermission(ctx, &pb.CheckPermissionRequest{
					Consistency: fullconsistency,
					Resource:    resourceToSpiceDBRef(namespace, lb1),
					Permission:  "loadbalancer_get",
					Subject:     &pb.SubjectReference{Object: resourceToSpiceDBRef(namespace, user2)},
				})
				require.Error(t, err)

				rb, err = e.CreateRoleBinding(ctx, user1, root, viewerRes, []types.RoleBindingSubject{{SubjectResource: group1}})
				require.NoError(t, err)

				return ctx
			},
			CheckFn: func(ctx context.Context, t *testing.T, _ testingx.TestResult[any]) {
				err := e.checkPermission(ctx, &pb.CheckPermissionRequest{
					Consistency: fullconsistency,
					Resource:    resourceToSpiceDBRef(namespace, lb1),
					Permission:  "loadbalancer_get",
					Subject:     &pb.SubjectReference{Object: resourceToSpiceDBRef(namespace, user2)},
				})
				assert.NoError(t, err)
			},
			// No cleanup
			Sync: true,
		},
		{
			Name: "GroupMembershipRemoval",
			SetupFn: func(ctx context.Context, t *testing.T) context.Context {
				err := e.checkPermission(ctx, &pb.CheckPermissionRequest{
					Consistency: fullconsistency,
					Resource:    resourceToSpiceDBRef(namespace, lb1),
					Permission:  "loadbalancer_get",
					Subject:     &pb.SubjectReference{Object: resourceToSpiceDBRef(namespace, user2)},
				})
				require.NoError(t, err)

				err = e.DeleteRelationships(ctx, types.Relationship{
					Resource: group1,
					Relation: "member",
					Subject:  user2,
				})
				require.NoError(t, err)

				return ctx
			},
			CheckFn: func(ctx context.Context, t *testing.T, _ testingx.TestResult[any]) {
				err := e.checkPermission(ctx, &pb.CheckPermissionRequest{
					Consistency: fullconsistency,
					Resource:    resourceToSpiceDBRef(namespace, lb1),
					Permission:  "loadbalancer_get",
					Subject:     &pb.SubjectReference{Object: resourceToSpiceDBRef(namespace, user2)},
				})
				assert.Error(t, err)
			},
			CleanupFn: func(ctx context.Context) {
				_ = e.CreateRelationships(ctx, []types.Relationship{{
					Resource: group1,
					Relation: "member",
					Subject:  user2,
				}})
			},
			Sync: true,
		},
		{
			Name: "RoleActionRemoval",
			SetupFn: func(ctx context.Context, t *testing.T) context.Context {
				err := e.checkPermission(ctx, &pb.CheckPermissionRequest{
					Consistency: fullconsistency,
					Resource:    resourceToSpiceDBRef(namespace, lb1),
					Permission:  "loadbalancer_get",
					Subject:     &pb.SubjectReference{Object: resourceToSpiceDBRef(namespace, user2)},
				})
				require.NoError(t, err)

				_, err = e.UpdateRoleV2(ctx, root, viewerRes, "lb_viewer", []string{"loadbalancer_list"})
				require.NoError(t, err)

				return ctx
			},
			CheckFn: func(ctx context.Context, t *testing.T, _ testingx.TestResult[any]) {
				err := e.checkPermission(ctx, &pb.CheckPermissionRequest{
					Consistency: fullconsistency,
					Resource:    resourceToSpiceDBRef(namespace, lb1),
					Permission:  "loadbalancer_get",
					Subject:     &pb.SubjectReference{Object: resourceToSpiceDBRef(namespace, user2)},
				})
				assert.Error(t, err)
			},
			CleanupFn: func(ctx context.Context) {
				_, _ = e.UpdateRoleV2(ctx, root, viewerRes, "lb_viewer", []string{"loadbalancer_list", "loadbalancer_get"})
			},
			Sync: true,
		},
		{
			Name: "DeleteRoleBinding",
			SetupFn: func(ctx context.Context, t *testing.T) context.Context {
				err := e.checkPermission(ctx, &pb.CheckPermissionRequest{
					Consistency: fullconsistency,
					Resource:    resourceToSpiceDBRef(namespace, lb1),
					Permission:  "loadbalancer_get",
					Subject:     &pb.SubjectReference{Object: resourceToSpiceDBRef(namespace, user2)},
				})
				require.NoError(t, err)

				rbRes, err := e.NewResourceFromID(rb.ID)
				require.NoError(t, err)
				err = e.DeleteRoleBinding(ctx, rbRes)
				require.NoError(t, err)

				return ctx
			},
			CheckFn: func(ctx context.Context, t *testing.T, _ testingx.TestResult[any]) {
				err := e.checkPermission(ctx, &pb.CheckPermissionRequest{
					Consistency: fullconsistency,
					Resource:    resourceToSpiceDBRef(namespace, lb1),
					Permission:  "loadbalancer_get",
					Subject:     &pb.SubjectReference{Object: resourceToSpiceDBRef(namespace, user2)},
				})
				assert.Error(t, err)
			},
			Sync: true,
		},
	}

	testFn := func(ctx context.Context, in any) testingx.TestResult[any] {
		return testingx.TestResult[any]{}
	}

	testingx.RunTests(ctx, t, tc, testFn)
}
