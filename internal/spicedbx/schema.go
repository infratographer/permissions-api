package spicedbx

import (
	"bytes"
	"text/template"

	"go.infratographer.com/permissions-api/internal/iapl"
	"go.infratographer.com/permissions-api/internal/types"
)

var schemaTemplate = template.Must(template.New("schema").Parse(`
{{- define "renderCondition" -}}
{{ $actionName := .Name }}
{{- range $index, $cond := .Conditions -}}
	{{- if $index }} + {{end}}
	{{- if $cond.RoleBinding }}{{ $actionName }}_rel{{ end }}
	{{- if $cond.RelationshipAction }}
		{{- $cond.RelationshipAction.Relation}}
		{{- if ne $cond.RelationshipAction.ActionName ""}}->{{ $cond.RelationshipAction.ActionName }}{{- end }}
	{{- end }}
{{- end }}
{{- end -}}

{{- define "renderConditionSet" -}}
{{ $actionName := .Name }}
{{- range $index, $conditionSet := .ConditionSets }}
	{{- if $index }} & {{end}}
	{{- if gt (len $conditionSet.Conditions) 1 -}} ( {{- end}}
	{{- range $index, $cond := .Conditions -}}
		{{- if $index }} + {{end}}
		{{- if $cond.RoleBinding }}{{ $actionName }}_rel{{ end }}
		{{- if $cond.RelationshipAction }}
			{{- $cond.RelationshipAction.Relation}}
			{{- if ne $cond.RelationshipAction.ActionName ""}}->{{ $cond.RelationshipAction.ActionName }}{{- end }}
		{{- end }}
	{{- end }}
	{{- if gt (len $conditionSet.Conditions) 1 -}} ) {{- end}}
	{{- end}}
{{- end -}}

{{- $namespace := .Namespace -}}
{{- range .ResourceTypes -}}
definition {{$namespace}}/{{.Name}} {
{{- range .Relationships }}
    relation {{.Relation}}: {{ range $index, $type := .Types -}}
			{{- if $index }} | {{end}}
			{{- $namespace}}/{{$type.Name}}
			{{- if $type.SubjectIdentifier}}:{{$type.SubjectIdentifier}}{{end}}
			{{- if $type.SubjectRelation}}#{{$type.SubjectRelation}}{{end}}
		{{- end }}
{{- end }}

{{- range .Actions }}
    permission {{ .Name }} = {{ if gt (len .Conditions) 0 }}
			{{- template "renderCondition" . }}
		{{- else if gt (len .ConditionSets) 0 }}
			{{- template "renderConditionSet" . }}
		{{- end}}
{{- end }}
}
{{end}}`))

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
