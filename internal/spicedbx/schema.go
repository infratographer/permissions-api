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

// GeneratedSchema generates the schema for a namespace
func GeneratedSchema(namespace string) string {
	policyDocument := iapl.PolicyDocument{
		ResourceTypes: []iapl.ResourceType{
			{
				Name:     "role",
				IDPrefix: "idenrol",
				Relationships: []iapl.Relationship{
					{
						Relation:       "subject",
						TargetTypeName: "subject",
					},
				},
			},
			{
				Name:     "user",
				IDPrefix: "idenusr",
			},
			{
				Name:     "client",
				IDPrefix: "idencli",
			},
			{
				Name:     "tenant",
				IDPrefix: "identen",
				Relationships: []iapl.Relationship{
					{
						Relation:       "tenant",
						TargetTypeName: "tenant",
					},
				},
			},
			{
				Name:     "loadbalancer",
				IDPrefix: "loadbal",
				Relationships: []iapl.Relationship{
					{
						Relation:       "tenant",
						TargetTypeName: "tenant",
					},
				},
			},
		},
		TypeAliases: []iapl.TypeAlias{
			{
				Name: "subject",
				ResourceTypeNames: []string{
					"user",
					"client",
				},
			},
		},
		Actions: []iapl.Action{
			{
				Name: "loadbalancer_create",
			},
			{
				Name: "loadbalancer_get",
			},
			{
				Name: "loadbalancer_list",
			},
			{
				Name: "loadbalancer_update",
			},
			{
				Name: "loadbalancer_delete",
			},
		},
		ActionBindings: []iapl.ActionBinding{
			{
				ActionName:       "loadbalancer_create",
				ResourceTypeName: "tenant",
				Conditions: []iapl.Condition{
					{
						RoleBinding: &iapl.ConditionRoleBinding{},
					},
					{
						RelationshipAction: &iapl.ConditionRelationshipAction{
							Relation:   "tenant",
							ActionName: "loadbalancer_create",
						},
					},
				},
			},
			{
				ActionName:       "loadbalancer_get",
				ResourceTypeName: "tenant",
				Conditions: []iapl.Condition{
					{
						RoleBinding: &iapl.ConditionRoleBinding{},
					},
					{
						RelationshipAction: &iapl.ConditionRelationshipAction{
							Relation:   "tenant",
							ActionName: "loadbalancer_get",
						},
					},
				},
			},
			{
				ActionName:       "loadbalancer_update",
				ResourceTypeName: "tenant",
				Conditions: []iapl.Condition{
					{
						RoleBinding: &iapl.ConditionRoleBinding{},
					},
					{
						RelationshipAction: &iapl.ConditionRelationshipAction{
							Relation:   "tenant",
							ActionName: "loadbalancer_update",
						},
					},
				},
			},
			{
				ActionName:       "loadbalancer_list",
				ResourceTypeName: "tenant",
				Conditions: []iapl.Condition{
					{
						RoleBinding: &iapl.ConditionRoleBinding{},
					},
					{
						RelationshipAction: &iapl.ConditionRelationshipAction{
							Relation:   "tenant",
							ActionName: "loadbalancer_list",
						},
					},
				},
			},
			{
				ActionName:       "loadbalancer_delete",
				ResourceTypeName: "tenant",
				Conditions: []iapl.Condition{
					{
						RoleBinding: &iapl.ConditionRoleBinding{},
					},
					{
						RelationshipAction: &iapl.ConditionRelationshipAction{
							Relation:   "tenant",
							ActionName: "loadbalancer_delete",
						},
					},
				},
			},
			{
				ActionName:       "loadbalancer_get",
				ResourceTypeName: "loadbalancer",
				Conditions: []iapl.Condition{
					{
						RoleBinding: &iapl.ConditionRoleBinding{},
					},
					{
						RelationshipAction: &iapl.ConditionRelationshipAction{
							Relation:   "tenant",
							ActionName: "loadbalancer_get",
						},
					},
				},
			},
			{
				ActionName:       "loadbalancer_update",
				ResourceTypeName: "loadbalancer",
				Conditions: []iapl.Condition{
					{
						RoleBinding: &iapl.ConditionRoleBinding{},
					},
					{
						RelationshipAction: &iapl.ConditionRelationshipAction{
							Relation:   "tenant",
							ActionName: "loadbalancer_update",
						},
					},
				},
			},
			{
				ActionName:       "loadbalancer_delete",
				ResourceTypeName: "loadbalancer",
				Conditions: []iapl.Condition{
					{
						RoleBinding: &iapl.ConditionRoleBinding{},
					},
					{
						RelationshipAction: &iapl.ConditionRelationshipAction{
							Relation:   "tenant",
							ActionName: "loadbalancer_delete",
						},
					},
				},
			},
		},
	}

	policy := iapl.NewPolicy(policyDocument)
	if err := policy.Validate(); err != nil {
		panic(err)
	}

	schema, err := GenerateSchema(namespace, policy.Schema())
	if err != nil {
		panic(err)
	}

	return schema
}
