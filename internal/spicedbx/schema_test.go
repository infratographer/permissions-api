package spicedbx

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"go.infratographer.com/permissions-api/internal/types"
)

func TestSchema(t *testing.T) {
	t.Parallel()

	type testInput struct {
		namespace     string
		resourceTypes []types.ResourceType
	}

	type testResult struct {
		success string
		err     error
	}

	type testCase struct {
		name    string
		input   testInput
		checkFn func(*testing.T, testResult)
	}

	resourceTypes := []types.ResourceType{
		{
			Name: "loadbalancer",
			Actions: []string{
				"get",
			},
			TenantActions: []string{
				"create",
			},
		},
		{
			Name: "port",
			Actions: []string{
				"get",
			},
			TenantActions: []string{
				"create",
			},
		},
	}

	schemaOutput := `definition foo/user {}

definition foo/client {}

definition foo/role {
    relation tenant: foo/tenant
    relation subject: foo/user | foo/client

    relation role_get_rel: foo/role#subject
    relation role_update_rel: foo/role#subject
    relation role_delete_rel: foo/role#subject

    permission role_get = role_get_rel + tenant->role_get
    permission role_update = role_update_rel + tenant->role_update
    permission role_delete = role_delete_rel + tenant->role_delete
}

definition foo/tenant {
    relation tenant: foo/tenant

    relation tenant_create_rel: foo/role#subject
    relation tenant_get_rel: foo/role#subject
    relation tenant_list_rel: foo/role#subject
    relation tenant_update_rel: foo/role#subject
    relation tenant_delete_rel: foo/role#subject

    permission role_create = role_create_rel + tenant->role_create
    permission role_get = role_get_rel + tenant->role_get
    permission role_list = role_list_rel + tenant->role_list
    permission role_update = role_update_rel + tenant->role_update
    permission role_delete = role_delete_rel + tenant->role_delete

    relation role_create_rel: foo/role#subject
    relation role_get_rel: foo/role#subject
    relation role_list_rel: foo/role#subject
    relation role_update_rel: foo/role#subject
    relation role_delete_rel: foo/role#subject

    permission role_create = role_create_rel + tenant->role_create
    permission role_get = role_get_rel + tenant->role_get
    permission role_list = role_list_rel + tenant->role_list
    permission role_update = role_update_rel + tenant->role_update
    permission role_delete = role_delete_rel + tenant->role_delete

    relation loadbalancer_get_rel: foo/role#subject

    permission loadbalancer_get = loadbalancer_get_rel + tenant->loadbalancer_get

    relation loadbalancer_create_rel: foo/role#subject

    permission loadbalancer_create = loadbalancer_create_rel + tenant->loadbalancer_create

    relation port_get_rel: foo/role#subject

    permission port_get = port_get_rel + tenant->port_get

    relation port_create_rel: foo/role#subject

    permission port_create = port_create_rel + tenant->port_create
}

definition foo/loadbalancer {
    relation tenant: foo/tenant

    relation loadbalancer_get_rel: foo/role#subject

    permission loadbalancer_get = loadbalancer_get_rel + tenant->loadbalancer_get
}

definition foo/port {
    relation tenant: foo/tenant

    relation port_get_rel: foo/role#subject

    permission port_get = port_get_rel + tenant->port_get
}
`

	testCases := []testCase{
		{
			name: "NoNamespace",
			input: testInput{
				namespace:     "",
				resourceTypes: resourceTypes,
			},
			checkFn: func(t *testing.T, res testResult) {
				assert.ErrorIs(t, res.err, ErrorNoNamespace)
				assert.Empty(t, res.success)
			},
		},
		{
			name: "SucccessNamespace",
			input: testInput{
				namespace:     "foo",
				resourceTypes: resourceTypes,
			},
			checkFn: func(t *testing.T, res testResult) {
				assert.NoError(t, res.err)
				assert.Equal(t, schemaOutput, res.success)
			},
		},
	}

	for i := range testCases {
		tc := testCases[i]
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			var result testResult

			result.success, result.err = GenerateSchema(tc.input.namespace, tc.input.resourceTypes)

			tc.checkFn(t, result)
		})
	}
}
