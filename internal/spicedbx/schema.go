package spicedbx

import (
	"strings"
)

func GeneratedSchema(prefix string) string {
	schema := `
definition PREFIX/subject {}

definition PREFIX/role {
    relation tenant: PREFIX/tenant
    relation subject: PREFIX/subject
}

definition PREFIX/tenant {
    relation parent: PREFIX/tenant

    relation loadbalancer_create_rel: PREFIX/role#subject
    relation loadbalancer_list_rel: PREFIX/role#subject
    relation loadbalancer_get_rel: PREFIX/role#subject
    relation loadbalancer_update_rel: PREFIX/role#subject
    relation loadbalancer_delete_rel: PREFIX/role#subject

    permission loadbalancer_create = loadbalancer_create_rel + parent->loadbalancer_create
    permission loadbalancer_list = loadbalancer_list_rel + parent->loadbalancer_list
    permission loadbalancer_get = loadbalancer_get_rel + parent->loadbalancer_get
    permission loadbalancer_update = loadbalancer_update_rel + parent->loadbalancer_update
    permission loadbalancer_delete = loadbalancer_delete_rel + parent->loadbalancer_delete
}

definition PREFIX/loadbalancer {
    relation tenant: PREFIX/tenant

    relation loadbalancer_get_rel: PREFIX/role#subject
    relation loadbalancer_update_rel: PREFIX/role#subject
    relation loadbalancer_delete_rel: PREFIX/role#subject

    permission loadbalancer_get = loadbalancer_get_rel + tenant->loadbalancer_get
    permission loadbalancer_update = loadbalancer_update_rel + tenant->loadbalancer_update
    permission loadbalancer_delete = loadbalancer_delete_rel + tenant->loadbalancer_delete
}
`

	if prefix != "" && !strings.HasSuffix(prefix, "/") {
		prefix += "/"
	}

	return strings.ReplaceAll(schema, "PREFIX/", prefix)

}
