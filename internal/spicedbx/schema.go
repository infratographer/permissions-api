package spicedbx

import (
	"strings"
)

func GeneratedSchema(prefix string) string {
	schema := `
definition PREFIX/global_scope {
	relation root_tenant_creator: PREFIX/user | PREFIX/service_account
	relation root_tenant_deleter: PREFIX/user | PREFIX/service_account
	relation global_scope_editor: PREFIX/user | PREFIX/service_account

	permission create_root_tenant = root_tenant_creator
	permission delete_root_tenant = root_tenant_deleter
	permission edit_global_scopes = global_scope_editor
}

definition PREFIX/user {}
definition PREFIX/service_account {}

definition PREFIX/role {
	relation tenant: PREFIX/tenant
	relation built_in_role: PREFIX/tenant
	relation actor: PREFIX/user | PREFIX/service_account

	permission add_user = tenant->assign_role
	permission delete = tenant->remove_role - built_in_role->remove_role
	permission add_permission = tenant->change_role - built_in_role->change_role
	permission remove_permission = tenant->change_role - built_in_role->change_role
}

definition PREFIX/tenant {
	relation parent_tenant: PREFIX/tenant
	relation permission_granter: PREFIX/role#actor
	relation role_editor: PREFIX/role#actor
	relation viewer: PREFIX/role#actor
	relation editor: PREFIX/role#actor
	relation deleter: PREFIX/role#actor

	permission inherited_member = parent_tenant->member
	permission direct_member = viewer + editor + deleter
	permission member = direct_member + inherited_member
	permission view = member
	permission edit = editor + parent_tenant->edit_child_tenants
	permission delete = deleter + parent_tenant->delete_child_tenants

	relation tenant_viewer: role#actor
	relation tenant_editor: role#actor
	relation tenant_creator: role#actor
	relation tenant_deleter: role#actor
	permission view_tenant = tenant_viewer + edit_tenant + create_tenant + delete_tenant + parent_tenant->view_tenant
	permission edit_tenant = tenant_editor + parent_tenant->edit_tenant
	permission create_tenant = tenant_creator + parent_tenant->create_tenant
	permission delete_tenant = tenant_deleter + parent_tenant->delete_tenant

	relation instance_creator: PREFIX/role#actor
	relation instance_deleter: PREFIX/role#actor
	relation instance_editor: PREFIX/role#actor
	relation instance_viewer: PREFIX/role#actor
	permission view_instance = instance_viewer + edit_instance + create_instance + delete_instance + parent_tenant->view_instance
	permission edit_instance = instance_editor + parent_tenant->edit_instance
	permission create_instance = instance_creator + parent_tenant->create_instance
	permission delete_instance = instance_deleter + parent_tenant->delete_instance

	relation instance_state_deleter: PREFIX/role#actor
	relation instance_state_setter: PREFIX/role#actor
	relation instance_state_viewer: PREFIX/role#actor
	permission view_instance_state = instance_state_viewer + view_instance + parent_tenant->view_instance_state
	permission edit_instance_state = instance_state_setter + parent_tenant->set_instance_state
	permission delete_instance_state = instance_state_deleter + parent_tenant->delete_instance_state

	relation ip_block_creator: PREFIX/role#actor
	relation ip_block_deleter: PREFIX/role#actor
	relation ip_block_editor: PREFIX/role#actor
	relation ip_block_viewer: PREFIX/role#actor
	permission view_ip_block = ip_block_viewer + edit_ip_block + create_ip_block + delete_ip_block + parent_tenant->view_ip_block
	permission edit_ip_block = ip_block_editor + parent_tenant->edit_ip_block
	permission create_ip_block = ip_block_creator + parent_tenant->create_ip_block
	permission delete_ip_block = ip_block_deleter + parent_tenant->delete_ip_block

	relation ip_address_creator: PREFIX/role#actor
	relation ip_address_deleter: PREFIX/role#actor
	relation ip_address_editor: PREFIX/role#actor
	relation ip_address_viewer: PREFIX/role#actor
	permission view_ip_address = ip_address_viewer + edit_ip_address + create_ip_address + delete_ip_address + parent_tenant->view_ip_address
	permission edit_ip_address = ip_address_editor + parent_tenant->edit_ip_address
	permission create_ip_address = ip_address_creator + parent_tenant->create_ip_address
	permission delete_ip_address = ip_address_deleter + parent_tenant->delete_ip_address
}

definition PREFIX/instance {
	relation tenant: PREFIX/tenant

	relation viewer: PREFIX/user | PREFIX/service_account | PREFIX/instance
	relation editor: PREFIX/user | PREFIX/service_account
	relation deleter: PREFIX/user | PREFIX/service_account
	relation ssh_key_signer: PREFIX/user | PREFIX/service_account

	permission view = viewer + editor + deleter + tenant->view_instance
	permission edit = editor + tenant->edit_instance
	permission delete = deleter + tenant->delete_instance
	permission sign_ssh_key = edit + ssh_key_signer + tenant->ssh_instance
}

definition PREFIX/ip_block {
	relation ip_block: PREFIX/ip_block
	relation tenant: PREFIX/tenant

	relation viewer: PREFIX/user | PREFIX/service_account
	relation editor: PREFIX/user | PREFIX/service_account
	relation deleter: PREFIX/user | PREFIX/service_account

	permission view = viewer + editor + deleter + ip_block->view + tenant->view_ip_block
	permission edit = editor + ip_block->edit + tenant->edit_ip_block
	permission delete = deleter + ip_block->delete + tenant->delete_ip_block
}

definition PREFIX/ip_address {
	relation parent_block: PREFIX/ip_block
	relation tenant: PREFIX/tenant
	relation assignment: PREFIX/instance

	relation viewer: PREFIX/user | PREFIX/service_account
	relation editor: PREFIX/user | PREFIX/service_account
	relation deleter: PREFIX/user | PREFIX/service_account

	permission view = viewer + editor + deleter + parent_block->view + tenant->view_ip_address + assignment->view
	permission edit = editor + parent_block->edit + tenant->edit_ip_address
	permission delete = deleter + parent_block->delete + tenant->delete_ip_address
}
`

	if prefix != "" && !strings.HasSuffix(prefix, "/") {
		prefix += "/"
	}

	return strings.ReplaceAll(schema, "PREFIX/", prefix)

}
