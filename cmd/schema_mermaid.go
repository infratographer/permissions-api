package cmd

import (
	"bytes"
	"fmt"
	"regexp"
	"text/template"

	"go.infratographer.com/permissions-api/internal/iapl"
	itypes "go.infratographer.com/permissions-api/internal/types"
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

func outputPolicyMermaid(dirPath string, markdown bool, filterTypes string, filterActions string) {
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

	if filterTypes != "" || filterActions != "" {
		if filterTypes == "" {
			filterTypes = ".*"
		}

		if filterActions == "" {
			filterActions = ".*"
		}

		policy = filterPolicy(policy, filterTypes, filterActions)
	}

	actions := map[string][]string{}
	relatedActions := map[string]map[string][]string{}

	for _, binding := range policy.ActionBindings {
		for _, cond := range binding.Conditions {
			if cond.RoleBinding != nil {
				actions[binding.TypeName] = append(actions[binding.TypeName], binding.ActionName)
			}

			if cond.RelationshipAction != nil && cond.RelationshipAction.ActionName != "" {
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

func filterPolicy(policy iapl.PolicyDocument, types, actions string) iapl.PolicyDocument {
	reTypes := regexp.MustCompile(types)
	reActions := regexp.MustCompile(actions)

	var (
		includeTypes   = map[string]map[string][]string{}
		includeActions = map[string]map[string]struct{}{}

		typesByName  = map[string]iapl.ResourceType{}
		unionsByName = map[string]map[string]iapl.ResourceType{}

		excludeTypes = map[string]struct{}{}
	)

	if policy.RBAC != nil {
		if policy.RBAC.RoleBindingResource.Name != "" {
			excludeTypes[policy.RBAC.RoleBindingResource.Name] = struct{}{}
		}
	}

	for _, rescType := range policy.ResourceTypes {
		typesByName[rescType.Name] = rescType

		if reTypes.MatchString(rescType.Name) {
			includeTypes[rescType.Name] = map[string][]string{}

			for _, rel := range rescType.Relationships {
				for _, targetType := range rel.TargetTypes {
					includeTypes[targetType.Name] = map[string][]string{}
				}
			}
		}
	}

	for _, rescType := range policy.Unions {
		unionsByName[rescType.Name] = map[string]iapl.ResourceType{}

		for _, targetType := range rescType.ResourceTypes {
			unionsByName[rescType.Name][targetType.Name] = typesByName[targetType.Name]
		}

		if reTypes.MatchString(rescType.Name) {
			includeTypes[rescType.Name] = map[string][]string{}
		}
	}

	for _, rescType := range policy.ResourceTypes {
		if _, ok := includeTypes[rescType.Name]; !ok {
			continue
		}

		for _, rel := range rescType.Relationships {
			for _, targetType := range rel.TargetTypes {
				includeTypes[rescType.Name][rel.Relation] = append(includeTypes[rescType.Name][rel.Relation], targetType.Name)
			}
		}
	}

	for _, binding := range policy.ActionBindings {
		if _, ok := includeTypes[binding.TypeName]; ok {
			if reActions.MatchString(binding.ActionName) {
				for _, cond := range binding.Conditions {
					if cond.RoleBinding != nil || cond.RoleBindingV2 != nil || cond.RelationshipAction != nil {
						includeAction(includeActions, binding.TypeName, binding.ActionName)
					}
					if cond.RelationshipAction != nil {
						for _, targetType := range includeTypes[binding.TypeName][cond.RelationshipAction.Relation] {
							if union, ok := unionsByName[targetType]; ok {
								for targetName := range union {
									includeAction(includeActions, targetName, cond.RelationshipAction.ActionName)
								}
							} else {
								includeAction(includeActions, targetType, cond.RelationshipAction.ActionName)
							}
						}
					}
				}
				for _, set := range binding.ConditionSets {
					for _, cond := range set.Conditions {
						if cond.RoleBinding != nil || cond.RoleBindingV2 != nil || cond.RelationshipAction != nil {
							includeAction(includeActions, binding.TypeName, binding.ActionName)
						}
						if cond.RelationshipAction != nil {
							for _, targetType := range includeTypes[binding.TypeName][cond.RelationshipAction.Relation] {
								if union, ok := unionsByName[targetType]; ok {
									for targetName := range union {
										includeAction(includeActions, targetName, cond.RelationshipAction.ActionName)
									}
								} else {
									includeAction(includeActions, targetType, cond.RelationshipAction.ActionName)
								}
							}
						}
					}
				}
			}
		}
	}

	var (
		filteredResources      []iapl.ResourceType
		filteredUnions         []iapl.Union
		filteredActionBindings []iapl.ActionBinding
	)

	for _, rescType := range policy.ResourceTypes {
		if _, ok := excludeTypes[rescType.Name]; ok {
			continue
		}

		include := false

		if _, ok := includeActions[rescType.Name]; ok {
			include = true
		} else {
			for _, related := range includeTypes[rescType.Name] {
				for _, relatedType := range related {
					if _, ok := includeTypes[relatedType]; ok {
						include = true
					}
				}
			}
		}

		if !include {
			continue
		}

		var filteredRelations []iapl.Relationship

		for _, relation := range rescType.Relationships {
			var filteredTargetTypes []itypes.TargetType

			for _, targetType := range relation.TargetTypes {
				if _, ok := excludeTypes[targetType.Name]; ok {
					continue
				}

				filteredTargetTypes = append(filteredTargetTypes, targetType)
			}

			if len(filteredTargetTypes) != 0 {
				relation.TargetTypes = filteredTargetTypes

				filteredRelations = append(filteredRelations, relation)
			}
		}

		rescType.Relationships = filteredRelations

		filteredResources = append(filteredResources, rescType)

		if rescType.RoleBindingV2 == nil {
			continue
		}

		for _, relation := range rescType.RoleBindingV2.InheritPermissionsFrom {
			for _, relTypeName := range includeTypes[rescType.Name][relation] {

				for _, binding := range policy.ActionBindings {
					if _, ok := includeTypes[relTypeName][binding.ActionName]; !ok {
						continue
					}

					binding.TypeName = relTypeName

					filteredActionBindings = append(filteredActionBindings, binding)
				}
			}
		}
	}

	for _, rescType := range policy.Unions {
		if _, ok := includeTypes[rescType.Name]; !ok {
			continue
		}

		var includedTargets []itypes.TargetType

		for _, target := range rescType.ResourceTypes {
			if _, ok := includeActions[target.Name]; ok {
				includedTargets = append(includedTargets, target)
			}
		}

		if len(includedTargets) != 0 {
			rescType.ResourceTypes = includedTargets

			filteredUnions = append(filteredUnions, rescType)
		}
	}

	for _, binding := range policy.ActionBindings {
		if union, ok := unionsByName[binding.TypeName]; ok {
			for targetName := range union {
				if _, ok := includeActions[targetName][binding.ActionName]; !ok {
					continue
				}

				typeBinding := binding

				typeBinding.TypeName = targetName

				filteredActionBindings = append(filteredActionBindings, typeBinding)
			}

			continue
		}

		if _, ok := includeActions[binding.TypeName][binding.ActionName]; !ok {
			continue
		}

		filteredActionBindings = append(filteredActionBindings, binding)
	}

	policy.RBAC = nil
	policy.ResourceTypes = filteredResources
	policy.Unions = filteredUnions
	policy.ActionBindings = filteredActionBindings

	return policy
}

func includeAction(state map[string]map[string]struct{}, typeName, actionName string) {
	if _, ok := state[typeName]; !ok {
		state[typeName] = map[string]struct{}{}
	}

	state[typeName][actionName] = struct{}{}
}
