package query

import (
	"context"
	"fmt"
	"testing"

	"go.infratographer.com/permissions-api/internal/iapl"
	"go.infratographer.com/permissions-api/internal/testingx"
	"go.infratographer.com/permissions-api/internal/types"

	v1 "github.com/authzed/authzed-go/proto/authzed/api/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.infratographer.com/x/gidx"
)

const PolicyDir = "../../policies"

/**
 * create hierarchical tenants and groups
 * this hierarchy ensures memberships, ownerships and inheritance are working
 * as intended
 *
 *                --------------
 *               | tnntten-root |
 *                --------------
 *                      |
 *                      |
 *                 -----------         ---------------          ------------------------
 *                | tnntten-a | ----- | idntgrp-admin | ------ | idntgrp-admin-subgroup |
 *                 -----------         ---------------          ------------------------
 *                 /         \
 *                /           \
 *         ------------    ------------
 *        | tnntten-b1 |  | tnntten-b2 |
 *         ------------    ------------
 *              |
 *              |
 *         -----------
 *        | tnntten-c |
 *         -----------
 *              |
 *              |
 *       ----------------
 *      | loadbal-test-a |
 *       ----------------
 */
func TestExamplePolicy(t *testing.T) {
	namespace := "infratographertests"
	ctx := context.Background()

	policy, err := iapl.NewPolicyFromDirectory(PolicyDir)
	require.NoError(t, err)

	e := testEngine(ctx, t, namespace, policy)

	// all actions
	allactions := []string{
		"iam_rolebinding_create",
		"iam_rolebinding_delete",
		"iam_rolebinding_get",
		"iam_rolebinding_list",
		"iam_rolebinding_update",
		"loadbalancer_create",
		"loadbalancer_delete",
		"loadbalancer_get",
		"loadbalancer_list",
		"loadbalancer_update",
		"role_create",
		"role_delete",
		"role_get",
		"role_list",
		"role_update",
	}

	iamactions := []string{
		"iam_rolebinding_create",
		"iam_rolebinding_delete",
		"iam_rolebinding_get",
		"iam_rolebinding_list",
		"iam_rolebinding_update",
		"role_create",
		"role_delete",
		"role_get",
		"role_list",
		"role_update",
	}

	lbactions := []string{
		"loadbalancer_create",
		"loadbalancer_delete",
		"loadbalancer_get",
		"loadbalancer_list",
		"loadbalancer_update",
	}

	lbactionsOnLB := []string{
		"loadbalancer_delete",
		"loadbalancer_get",
		"loadbalancer_update",
	}

	// users
	superuser := types.Resource{Type: "user", ID: "idntusr-superuser"}
	haroldadmin := types.Resource{Type: "user", ID: "idntusr-harold-admin"}
	theotheradmin := types.Resource{Type: "user", ID: "idntusr-the-other-admin"}

	// tenants
	tnnttenroot := types.Resource{Type: "tenant", ID: gidx.PrefixedID("tnntten-root")}
	tnnttena := types.Resource{Type: "tenant", ID: gidx.PrefixedID("tnntten-a")}
	tnnttenb1 := types.Resource{Type: "tenant", ID: gidx.PrefixedID("tnntten-b1")}
	tnnttenb2 := types.Resource{Type: "tenant", ID: gidx.PrefixedID("tnntten-b2")}
	tnnttenc := types.Resource{Type: "tenant", ID: gidx.PrefixedID("tnntten-c")}

	// groups
	groupadmin := types.Resource{Type: "group", ID: "idntgrp-admin"}
	groupadminsubgroup := types.Resource{Type: "group", ID: "idntgrp-admin-subgroup"}

	// resources
	lbtesta := types.Resource{Type: "loadbalancer", ID: "loadbal-test-a"}

	// create hierarchical tenants and groups
	err = e.CreateRelationships(ctx, []types.Relationship{
		// tenants
		{
			Resource: tnnttena,
			Relation: "parent",
			Subject:  tnnttenroot,
		},
		{
			Resource: tnnttenb1,
			Relation: "parent",
			Subject:  tnnttena,
		},
		{
			Resource: tnnttenb2,
			Relation: "parent",
			Subject:  tnnttena,
		},
		{
			Resource: tnnttenc,
			Relation: "parent",
			Subject:  tnnttenb1,
		},
		// groups
		{
			Resource: groupadmin,
			Relation: "parent",
			Subject:  tnnttena,
		},
		{
			Resource: groupadminsubgroup,
			Relation: "parent",
			Subject:  groupadmin,
		},
		{
			Resource: groupadmin,
			Relation: "subgroup",
			Subject:  groupadminsubgroup,
		},
		{
			Resource: groupadmin,
			Relation: "direct_member",
			Subject:  theotheradmin,
		},
		{
			Resource: groupadminsubgroup,
			Relation: "direct_member",
			Subject:  haroldadmin,
		},
		// resources
		{
			Resource: lbtesta,
			Relation: "owner",
			Subject:  tnnttenc,
		},
	})
	require.NoError(t, err)

	// create roles
	superadmin, err := e.CreateRoleV2(ctx, superuser, tnnttenroot, "superuser", allactions)
	require.NoError(t, err)

	iamadmin, err := e.CreateRoleV2(ctx, superuser, tnnttena, "iam_admin", iamactions)
	require.NoError(t, err)

	lbadmin, err := e.CreateRoleV2(ctx, superuser, tnnttenroot, "lb_admin", lbactions)
	require.NoError(t, err)

	tc := []testingx.TestCase[any, any]{
		{
			Name: "superuser can do anything",
			SetupFn: func(ctx context.Context, t *testing.T) context.Context {
				role := types.Resource{Type: "role", ID: superadmin.ID}
				_, err := e.CreateRoleBinding(ctx, superuser, tnnttenroot, role, []types.RoleBindingSubject{{SubjectResource: superuser}})
				require.NoError(t, err)

				return ctx
			},
			CheckFn: func(ctx context.Context, t *testing.T, tr testingx.TestResult[any]) {
				res := []types.Resource{tnnttenroot, tnnttena, tnnttenb1, tnnttenb2, tnnttenc}

				for _, r := range res {
					for _, a := range allactions {
						err := e.checkPermission(ctx, &v1.CheckPermissionRequest{
							Consistency: &v1.Consistency{Requirement: &v1.Consistency_FullyConsistent{FullyConsistent: true}},
							Resource:    resourceToSpiceDBRef(e.namespace, r),
							Subject:     &v1.SubjectReference{Object: resourceToSpiceDBRef(e.namespace, superuser)},
							Permission:  a,
						})
						assert.NoError(t, err, fmt.Sprintf("superuser should have permission %s on %s", a, r.ID))
					}
				}

				for _, a := range lbactionsOnLB {
					err := e.checkPermission(ctx, &v1.CheckPermissionRequest{
						Consistency: &v1.Consistency{Requirement: &v1.Consistency_FullyConsistent{FullyConsistent: true}},
						Resource:    resourceToSpiceDBRef(e.namespace, lbtesta),
						Subject:     &v1.SubjectReference{Object: resourceToSpiceDBRef(e.namespace, superuser)},
						Permission:  a,
					})
					assert.NoError(t, err, fmt.Sprintf("superuser should have permission %s on %s", a, lbtesta.ID))
				}
			},
		},
		{
			Name: "the other admin can manage lbs under tnntten-a",
			Sync: true,
			SetupFn: func(ctx context.Context, t *testing.T) context.Context {
				role := types.Resource{Type: "role", ID: lbadmin.ID}
				_, err := e.CreateRoleBinding(ctx, superuser, tnnttena, role, []types.RoleBindingSubject{{SubjectResource: groupadmin}})
				require.NoError(t, err)

				return ctx
			},
			CheckFn: func(ctx context.Context, t *testing.T, tr testingx.TestResult[any]) {
				res := []types.Resource{tnnttena, tnnttenb1, tnnttenb2, tnnttenc}

				forbidden := iamactions
				allowed := lbactions

				// the other admin has no permissions on tnntten-root
				for _, r := range res {
					for _, a := range allowed {
						err := e.checkPermission(ctx, &v1.CheckPermissionRequest{
							Consistency: &v1.Consistency{Requirement: &v1.Consistency_FullyConsistent{FullyConsistent: true}},
							Resource:    resourceToSpiceDBRef(e.namespace, r),
							Subject:     &v1.SubjectReference{Object: resourceToSpiceDBRef(e.namespace, theotheradmin)},
							Permission:  a,
						})
						assert.NoError(t, err, fmt.Sprintf("the other admin should have permission %s on %s", a, r.ID))
					}
					for _, a := range forbidden {
						err := e.checkPermission(ctx, &v1.CheckPermissionRequest{
							Consistency: &v1.Consistency{Requirement: &v1.Consistency_FullyConsistent{FullyConsistent: true}},
							Resource:    resourceToSpiceDBRef(e.namespace, r),
							Subject:     &v1.SubjectReference{Object: resourceToSpiceDBRef(e.namespace, theotheradmin)},
							Permission:  a,
						})
						assert.Error(t, err, fmt.Sprintf("the other admin should not have permission %s on %s", a, r.ID))
					}
				}

				for _, a := range lbactionsOnLB {
					err := e.checkPermission(ctx, &v1.CheckPermissionRequest{
						Consistency: &v1.Consistency{Requirement: &v1.Consistency_FullyConsistent{FullyConsistent: true}},
						Resource:    resourceToSpiceDBRef(e.namespace, lbtesta),
						Subject:     &v1.SubjectReference{Object: resourceToSpiceDBRef(e.namespace, haroldadmin)},
						Permission:  a,
					})
					assert.NoError(t, err, fmt.Sprintf(" should have permission %s on %s", a, lbtesta.ID))
				}
			},
		},
		{
			// lb-admin permissions should be inherited from group-admin to group-admin-subgroup
			// iam-admin + lb-admin = superuser
			Name: "harold-admin can do anything under tnntten-a",
			Sync: true,
			SetupFn: func(ctx context.Context, t *testing.T) context.Context {
				role := types.Resource{Type: "role", ID: iamadmin.ID}
				_, err := e.CreateRoleBinding(ctx, superuser, tnnttena, role, []types.RoleBindingSubject{{SubjectResource: groupadminsubgroup}})
				require.NoError(t, err)

				return ctx
			},
			CheckFn: func(ctx context.Context, t *testing.T, tr testingx.TestResult[any]) {
				nopermRes := tnnttenroot
				res := []types.Resource{tnnttena, tnnttenb1, tnnttenb2, tnnttenc}

				// harold-admin has no permissions on tnntten-root
				for _, a := range allactions {
					err := e.checkPermission(ctx, &v1.CheckPermissionRequest{
						Consistency: &v1.Consistency{Requirement: &v1.Consistency_FullyConsistent{FullyConsistent: true}},
						Resource:    resourceToSpiceDBRef(e.namespace, nopermRes),
						Subject:     &v1.SubjectReference{Object: resourceToSpiceDBRef(e.namespace, haroldadmin)},
						Permission:  a,
					})
					assert.Error(t, err, fmt.Sprintf("harold-admin should have no permission %s", nopermRes.ID))
				}

				for _, r := range res {
					for _, a := range allactions {
						err := e.checkPermission(ctx, &v1.CheckPermissionRequest{
							Consistency: &v1.Consistency{Requirement: &v1.Consistency_FullyConsistent{FullyConsistent: true}},
							Resource:    resourceToSpiceDBRef(e.namespace, r),
							Subject:     &v1.SubjectReference{Object: resourceToSpiceDBRef(e.namespace, haroldadmin)},
							Permission:  a,
						})
						assert.NoError(t, err, fmt.Sprintf("harold-admin should have permission %s on %s", a, r.ID))
					}
				}

				for _, a := range lbactionsOnLB {
					err := e.checkPermission(ctx, &v1.CheckPermissionRequest{
						Consistency: &v1.Consistency{Requirement: &v1.Consistency_FullyConsistent{FullyConsistent: true}},
						Resource:    resourceToSpiceDBRef(e.namespace, lbtesta),
						Subject:     &v1.SubjectReference{Object: resourceToSpiceDBRef(e.namespace, haroldadmin)},
						Permission:  a,
					})
					assert.NoError(t, err, fmt.Sprintf("harold-admin should have permission %s on %s", a, lbtesta.ID))
				}
			},
		},
		{
			Name: "iam-admin cannot be bind on tnntten-root",
			CheckFn: func(ctx context.Context, t *testing.T, tr testingx.TestResult[any]) {
				role := types.Resource{Type: "role", ID: iamadmin.ID}
				_, err := e.CreateRoleBinding(ctx, superuser, tnnttenroot, role, []types.RoleBindingSubject{{SubjectResource: groupadminsubgroup}})
				assert.Error(t, err)
				assert.ErrorIs(t, err, ErrRoleNotFound)
			},
		},
	}

	testFn := func(ctx context.Context, in any) testingx.TestResult[any] {
		return testingx.TestResult[any]{}
	}

	testingx.RunTests(ctx, t, tc, testFn)
}
