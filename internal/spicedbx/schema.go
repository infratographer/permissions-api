package spicedbx

import (
	"bytes"
	"text/template"

	"go.infratographer.com/permissions-api/internal/types"
)

var (
	schemaTemplate = template.Must(template.New("schema").Parse(`
{{$prefix := .Prefix}}
definition {{$prefix}}/subject {}

definition {{$prefix}}/role {
    relation tenant: {{$prefix}}/tenant
    relation subject: {{$prefix}}/subject
}

definition {{$prefix}}/tenant {
    relation tenant: {{$prefix}}/tenant
{{- range .ResourceTypes -}}
{{$typeName := .Name}}
{{range .TenantActions}}
    relation {{$typeName}}_{{.}}_rel: {{$prefix}}/role#subject
{{- end}}
{{range .TenantActions}}
    permission {{$typeName}}_{{.}} = {{$typeName}}_{{.}}_rel + tenant->{{$typeName}}_{{.}}
{{- end}}
{{- end}}
}
{{range .ResourceTypes -}}
{{$typeName := .Name}}
definition {{$prefix}}/{{$typeName}} {
    relation tenant: {{$prefix}}/tenant
{{range .TenantActions}}
    relation {{$typeName}}_{{.}}_rel: {{$prefix}}/role#subject
{{- end}}
{{range .TenantActions}}
    permission {{$typeName}}_{{.}} = {{$typeName}}_{{.}}_rel + tenant->{{$typeName}}_{{.}}
{{- end}}
}
{{end}}
`))
)

func GenerateSchema(namespace string, resourceTypes []types.ResourceType) (string, error) {
	var data struct {
		Prefix        string
		ResourceTypes []types.ResourceType
	}

	data.Prefix = namespace
	data.ResourceTypes = resourceTypes

	var out bytes.Buffer

	err := schemaTemplate.Execute(&out, data)
	if err != nil {
		return "", err
	}

	return out.String(), nil
}

func GeneratedSchema(prefix string) string {
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

	schema, err := GenerateSchema(prefix, resourceTypes)
	if err != nil {
		panic(err)
	}

	return schema
}
