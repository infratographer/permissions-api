package spicedbx

import (
	"bytes"
	"text/template"

	"go.infratographer.com/permissions-api/internal/iapl"
	"go.infratographer.com/permissions-api/internal/types"
)

var (
	schemaTemplate = template.Must(template.New("schema").Parse(`
{{- $namespace := .Namespace -}}
{{- range .ResourceTypes -}}
definition {{$namespace}}/{{.Name}} {
{{- range .Relationships }}
    relation {{.Relation}}: {{ range $index, $typeName := .Types -}}{{ if $index }} | {{end}}{{$namespace}}/{{$typeName}}{{- end }}
{{- end }}

{{- range .Actions }}
    relation {{.Name}}_rel: {{ $namespace }}/role#subject
{{- end }}

{{- range .Actions }}
{{- $actionName := .Name }}
    permission {{ $actionName }} = {{ range $index, $cond := .Conditions -}}{{ if $index }} + {{end}}{{ if $cond.RoleBinding }}{{ $actionName }}_rel{{ end }}{{ if $cond.RelationshipAction }}{{ $cond.RelationshipAction.Relation}}->{{ $cond.RelationshipAction.ActionName }}{{ end }}{{- end }}
{{- end }}
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

// GeneratedSchema produces a namespaced SpiceDB schema based on the default IAPL policy.
func GeneratedSchema(namespace string) string {
	policy := iapl.DefaultPolicy()

	schema, err := GenerateSchema(namespace, policy.Schema())
	if err != nil {
		panic(err)
	}

	return schema
}
