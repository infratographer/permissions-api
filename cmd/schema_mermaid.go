package cmd

import (
	"bytes"
	"fmt"
	"text/template"

	"go.infratographer.com/permissions-api/internal/iapl"
)

var (
	mermaidTemplate = `erDiagram
{{- if ne .RBAC nil}}
	{{ .RBAC.RoleBindingResource }} }o--o{ {{ .RBAC.RoleResource }} : role
	{{- range $subj := .RBAC.RoleBindingSubjects }}
	{{ $.RBAC.RoleBindingResource }} }o--o{ {{ $subj.Name }} : subject
	{{- end }}
{{- end }}

{{- range $resource := .ResourceTypes }}
	{{ $resource.Name }} {
		id_prefix {{ $resource.IDPrefix }}
		{{- range $action := index $.Actions $resource.Name }}
		action {{ $action }}
		{{- end }}
		{{- range $relation, $actions := index $.RelatedActions $resource.Name }}
		{{- range $action := $actions }}
		{{ $relation }}_action {{ $action }}
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
		{{- range $action := index $.Actions $union.Name }}
		action {{ $action }}
		{{- end }}
		{{- range $relation, $actions := index $.RelatedActions $union.Name }}
		{{- range $action := $actions }}
		{{ $relation }}_action {{ $action }}
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
	ResourceTypes  []iapl.ResourceType
	Unions         []iapl.Union
	Actions        map[string][]string
	RelatedActions map[string]map[string][]string
	RBAC           *iapl.RBAC
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

	actions := map[string][]string{}
	relatedActions := map[string]map[string][]string{}

	for _, binding := range policy.ActionBindings {
		for _, cond := range binding.Conditions {
			if cond.RoleBinding != nil {
				actions[binding.TypeName] = append(actions[binding.TypeName], binding.ActionName)
			}

			if cond.RelationshipAction != nil {
				if _, ok := relatedActions[binding.TypeName]; !ok {
					relatedActions[binding.TypeName] = make(map[string][]string)
				}

				relatedActions[binding.TypeName][cond.RelationshipAction.Relation] = append(relatedActions[binding.TypeName][cond.RelationshipAction.Relation], cond.RelationshipAction.ActionName)
			}
		}
	}

	ctx := mermaidContext{
		ResourceTypes:  policy.ResourceTypes,
		Unions:         policy.Unions,
		Actions:        actions,
		RelatedActions: relatedActions,
		RBAC:           nil,
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
