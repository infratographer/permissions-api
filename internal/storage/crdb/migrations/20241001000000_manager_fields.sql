-- +goose Up

ALTER TABLE roles ADD COLUMN IF NOT EXISTS manager CHARACTER VARYING(128) NOT NULL DEFAULT '';
ALTER TABLE rolebindings ADD COLUMN IF NOT EXISTS manager CHARACTER VARYING(128) NOT NULL DEFAULT '';

CREATE INDEX IF NOT EXISTS "roles_manager_resource_id" ON "roles" ("manager", "resource_id");
CREATE INDEX IF NOT EXISTS "rolebindings_manager_resource_id" ON "rolebindings" ("manager", "resource_id");
