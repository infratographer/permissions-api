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
			TenantActions: []string{
				"create",
				"get",
			},
		},
		{
			Name: "port",
			TenantActions: []string{
				"create",
				"get",
			},
		},
	}

	schemaOutput := `definition foo/subject {}

definition foo/role {
    relation tenant: foo/tenant
    relation subject: foo/subject
}

definition foo/tenant {
    relation tenant: foo/tenant

    relation loadbalancer_create_rel: foo/role#subject
    relation loadbalancer_get_rel: foo/role#subject

    permission loadbalancer_create = loadbalancer_create_rel + tenant->loadbalancer_create
    permission loadbalancer_get = loadbalancer_get_rel + tenant->loadbalancer_get

    relation port_create_rel: foo/role#subject
    relation port_get_rel: foo/role#subject

    permission port_create = port_create_rel + tenant->port_create
    permission port_get = port_get_rel + tenant->port_get
}

definition foo/loadbalancer {
    relation tenant: foo/tenant

    relation loadbalancer_create_rel: foo/role#subject
    relation loadbalancer_get_rel: foo/role#subject

    permission loadbalancer_create = loadbalancer_create_rel + tenant->loadbalancer_create
    permission loadbalancer_get = loadbalancer_get_rel + tenant->loadbalancer_get
}

definition foo/port {
    relation tenant: foo/tenant

    relation port_create_rel: foo/role#subject
    relation port_get_rel: foo/role#subject

    permission port_create = port_create_rel + tenant->port_create
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
