package spicedbx

import (
	"bytes"
	"text/template"

	"go.infratographer.com/permissions-api/internal/types"
)

var (
	schemaTemplate = template.Must(template.New("schema").Parse(`
{{- $namespace := .Namespace -}}
definition {{$namespace}}/user {}

definition {{$namespace}}/client {}

definition {{$namespace}}/role {
    relation tenant: {{$namespace}}/tenant
    relation subject: {{$namespace}}/user | {{$namespace}}/client

    relation role_get_rel: {{$namespace}}/role#subject
    relation role_update_rel: {{$namespace}}/role#subject
    relation role_delete_rel: {{$namespace}}/role#subject

    permission role_get = role_get_rel + tenant->role_get
    permission role_update = role_update_rel + tenant->role_update
    permission role_delete = role_delete_rel + tenant->role_delete
}

definition {{$namespace}}/tenant {
    relation tenant: {{$namespace}}/tenant

    relation tenant_create_rel: {{$namespace}}/role#subject
    relation tenant_get_rel: {{$namespace}}/role#subject
    relation tenant_list_rel: {{$namespace}}/role#subject
    relation tenant_update_rel: {{$namespace}}/role#subject
    relation tenant_delete_rel: {{$namespace}}/role#subject

    permission tenant_create = tenant_create_rel + tenant->tenant_create
    permission tenant_get = tenant_get_rel + tenant->tenant_get
    permission tenant_list = tenant_list_rel + tenant->tenant_list
    permission tenant_update = tenant_update_rel + tenant->tenant_update
    permission tenant_delete = tenant_delete_rel + tenant->tenant_delete

    relation role_create_rel: {{$namespace}}/role#subject
    relation role_get_rel: {{$namespace}}/role#subject
    relation role_list_rel: {{$namespace}}/role#subject
    relation role_update_rel: {{$namespace}}/role#subject
    relation role_delete_rel: {{$namespace}}/role#subject

    permission role_create = role_create_rel + tenant->role_create
    permission role_get = role_get_rel + tenant->role_get
    permission role_list = role_list_rel + tenant->role_list
    permission role_update = role_update_rel + tenant->role_update
    permission role_delete = role_delete_rel + tenant->role_delete

{{- range .ResourceTypes -}}
{{$typeName := .Name}}
{{range .Actions}}
    relation {{$typeName}}_{{.}}_rel: {{$namespace}}/role#subject
{{- end}}
{{range .Actions}}
    permission {{$typeName}}_{{.}} = {{$typeName}}_{{.}}_rel + tenant->{{$typeName}}_{{.}}
{{- end}}
{{range .TenantActions}}
    relation {{$typeName}}_{{.}}_rel: {{$namespace}}/role#subject
{{- end}}
{{range .TenantActions}}
    permission {{$typeName}}_{{.}} = {{$typeName}}_{{.}}_rel + tenant->{{$typeName}}_{{.}}
{{- end}}
{{- end}}
}
{{range .ResourceTypes -}}
{{$typeName := .Name}}
definition {{$namespace}}/{{$typeName}} {
    relation tenant: {{$namespace}}/tenant
{{range .Actions}}
    relation {{$typeName}}_{{.}}_rel: {{$namespace}}/role#subject
{{- end}}
{{range .Actions}}
    permission {{$typeName}}_{{.}} = {{$typeName}}_{{.}}_rel + tenant->{{$typeName}}_{{.}}
{{- end}}
}
{{end}}`))
)

// GenerateSchema generates the spicedb schema from the template
func GenerateSchema(namespace string, resourceTypes []types.ResourceType) (string, error) {
	if namespace == "" {
		return "", ErrorNoNamespace
	}

	var data struct {
		Namespace     string
		ResourceTypes []types.ResourceType
	}

	data.Namespace = namespace
	data.ResourceTypes = resourceTypes

	var out bytes.Buffer

	err := schemaTemplate.Execute(&out, data)
	if err != nil {
		return "", err
	}

	return out.String(), nil
}

// GeneratedSchema generated the schema for a namespace
func GeneratedSchema(namespace string) string {
	resourceTypes := []types.ResourceType{
		{
			Name: "loadbalancer",
			Actions: []string{
				"get",
				"update",
				"delete",
			},
			TenantActions: []string{
				"create",
				"list",
			},
		},
	}

	schema, err := GenerateSchema(namespace, resourceTypes)
	if err != nil {
		panic(err)
	}

	return schema
}
