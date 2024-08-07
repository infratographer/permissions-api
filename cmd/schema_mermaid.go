package cmd

import (
	"bytes"
	"fmt"
	"slices"
	"text/template"

	"go.infratographer.com/permissions-api/internal/iapl"
)

var (
	mermaidTemplate = `erDiagram
{{- if ne .RBAC nil}}
	{{ .RBAC.RoleBindingResource.Name }} }o--o{ {{ .RBAC.RoleResource.Name }} : role
	{{- range $subj := .RBAC.RoleBindingSubjects }}
	{{ $.RBAC.RoleBindingResource.Name }} }o--o{ {{ $subj.Name }} : subject
	{{- end }}
{{- end }}

{{- range $resource := .ResourceTypes }}
	{{ $resource.Name }} {
		id_prefix {{ $resource.IDPrefix }}
		{{- range $action, $relations := index $.Actions $resource.Name }}
		action {{ $action }}
			{{- $quoted := false }}
			{{- with index $relations "!!SELF!!" }}
				{{- " \"self" }}
				{{- $quoted = true }}
			{{- end }}
			{{- range $relation := $.RelationOrder }}
				{{- with index $relations $relation }}
					{{- range $i, $rel_action := . }}
						{{- if not $quoted }}
							{{- " \"" }}
							{{- $quoted = true }}
						{{- else }}
							{{- " | " }}
						{{- end }}

						{{- $relation }}>{{- $rel_action }}
					{{- end }}
				{{- end }}
			{{- end }}
			{{- if $quoted }}
				{{- "\"" }}
			{{- end }}
		{{- end }}
	}
    {{- range $rel := $resource.Relationships }}

	{{- range $target := $rel.TargetTypes }}
	{{ $resource.Name }} }o--o{ {{ $target.Name -}} : {{ $rel.Relation -}}
	{{- end }}

	{{- end }}
{{- end }}
{{- range $union := .Unions }}
	{{ $union.Name }} {
		{{- range $action, $relations := index $.Actions $union.Name }}
		action {{ $action }}
			{{- $quoted := false }}
			{{- with index $relations "!!SELF!!" }}
				{{- " \"self" }}
				{{- $quoted = true }}
			{{- end }}
			{{- range $relation := $.RelationOrder }}
				{{- with index $relations $relation }}
					{{- range $i, $rel_action := . }}
						{{- if not $quoted }}
							{{- " \"" }}
							{{- $quoted = true }}
						{{- else }}
							{{- " | " }}
						{{- end }}

						{{- $relation }}>{{- $rel_action }}
					{{- end }}
				{{- end }}
			{{- end }}
			{{- if $quoted }}
				{{- "\"" }}
			{{- end }}
		{{- end }}
	}
	{{- range $typ := $union.ResourceTypes }}
	{{ $union.Name }} ||--|| {{ $typ.Name -}} : alias
	{{- end}}

{{- end }}
`

	mermaidTmpl = template.Must(template.New("mermaid").Parse(mermaidTemplate))
)

type mermaidContext struct {
	ResourceTypes []iapl.ResourceType
	Unions        []iapl.Union
	Actions       map[string]map[string]map[string][]string
	RelationOrder []string
	RBAC          *iapl.RBAC
}

func outputPolicyMermaid(dirPath string, markdown bool) {
	var (
		policy iapl.PolicyDocument
		err    error
	)

	if dirPath != "" {
		policy, err = iapl.LoadPolicyDocumentFromDirectory(dirPath)
		if err != nil {
			logger.Fatalw("failed to load policy documents", "error", err)
		}
	} else {
		policy = iapl.DefaultPolicyDocument()
	}

	actions := map[string]map[string]map[string][]string{}
	relations := []string{}

	for _, binding := range policy.ActionBindings {
		for _, cond := range binding.Conditions {
			if cond.RoleBinding != nil {
				if _, ok := actions[binding.TypeName]; !ok {
					actions[binding.TypeName] = make(map[string]map[string][]string)
				}

				if _, ok := actions[binding.TypeName][binding.ActionName]; !ok {
					actions[binding.TypeName][binding.ActionName] = make(map[string][]string)
				}

				actions[binding.TypeName][binding.ActionName]["!!SELF!!"] = append(actions[binding.TypeName][binding.ActionName]["!!SELF!!"], binding.ActionName)
			}

			if cond.RelationshipAction != nil {
				if _, ok := actions[binding.TypeName]; !ok {
					actions[binding.TypeName] = make(map[string]map[string][]string)
				}

				if _, ok := actions[binding.TypeName][binding.ActionName]; !ok {
					actions[binding.TypeName][binding.ActionName] = make(map[string][]string)
				}

				actions[binding.TypeName][binding.ActionName][cond.RelationshipAction.Relation] = append(actions[binding.TypeName][binding.ActionName][cond.RelationshipAction.Relation], cond.RelationshipAction.ActionName)

				if !slices.Contains(relations, cond.RelationshipAction.Relation) {
					relations = append(relations, cond.RelationshipAction.Relation)
				}
			}
		}
	}

	slices.Sort(relations)

	ctx := mermaidContext{
		ResourceTypes: policy.ResourceTypes,
		Unions:        policy.Unions,
		Actions:       actions,
		RelationOrder: relations,
		RBAC:          nil,
	}

	if policy.RBAC != nil {
		ctx.RBAC = policy.RBAC
	}

	var out bytes.Buffer

	if err := mermaidTmpl.Execute(&out, ctx); err != nil {
		logger.Fatalw("failed to render mermaid chart for policy", "error", err)
	}

	if markdown {
		fmt.Printf("```mermaid\n%s\n```\n", out.String())

		return
	}

	fmt.Println(out.String())
}
