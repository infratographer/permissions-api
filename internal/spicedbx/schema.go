package spicedbx

import (
	"bytes"
	"text/template"

	"go.infratographer.com/permissions-api/internal/types"
)

var (
	schemaTemplate = template.Must(template.New("schema").Parse(`
{{- $namespace := .Namespace -}}
definition {{$namespace}}/subject {}

definition {{$namespace}}/role {
    relation tenant: {{$namespace}}/tenant
    relation subject: {{$namespace}}/subject
}

definition {{$namespace}}/tenant {
    relation tenant: {{$namespace}}/tenant
{{- range .ResourceTypes -}}
{{$typeName := .Name}}
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
{{range .TenantActions}}
    relation {{$typeName}}_{{.}}_rel: {{$namespace}}/role#subject
{{- end}}
{{range .TenantActions}}
    permission {{$typeName}}_{{.}} = {{$typeName}}_{{.}}_rel + tenant->{{$typeName}}_{{.}}
{{- end}}
}
{{end}}`))
)

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

func GeneratedSchema(namespace string) string {
	resourceTypes := []types.ResourceType{
		{
			Name: "loadbalancer",
			TenantActions: []string{
				"create",
				"get",
				"list",
				"update",
				"delete",
			},
		},
	}

	schema, err := GenerateSchema(namespace, resourceTypes)
	if err != nil {
		panic(err)
	}

	return schema
}
