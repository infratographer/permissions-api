package cmd

import (
	"bytes"
	"fmt"
	"os"
	"text/template"

	"go.infratographer.com/permissions-api/internal/iapl"
	"gopkg.in/yaml.v3"
)

var (
	mermaidTemplate = `erDiagram
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
	{{- range $targetName := $rel.TargetTypeNames }}
	{{ $resource.Name }} }o--o{ {{ $targetName }} : {{ $rel.Relation }}
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
	{{- range $typ := $union.ResourceTypeNames }}
	{{ $union.Name }} ||--|| {{ $typ }} : alias
	{{- end }}
{{- end }}`

	mermaidTmpl = template.Must(template.New("mermaid").Parse(mermaidTemplate))
)

type mermaidContext struct {
	ResourceTypes  []iapl.ResourceType
	Unions         []iapl.Union
	Actions        map[string][]string
	RelatedActions map[string]map[string][]string
}

func outputPolicyMermaid(filePath string, markdown bool) {
	var policy iapl.PolicyDocument

	if filePath != "" {
		file, err := os.Open(filePath)
		if err != nil {
			logger.Fatalw("failed to open policy document file", "error", err)
		}

		defer file.Close()

		if err := yaml.NewDecoder(file).Decode(&policy); err != nil {
			logger.Fatalw("failed to load policy document file", "error", err)
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
