-- +goose NO TRANSACTION
-- +goose Up

-- create "roles" table
CREATE TABLE "roles" (
  "id" character varying NOT NULL,
  "name" character varying(64) NOT NULL,
  "resource_id" character varying NOT NULL,
  "created_by" character varying NOT NULL,
  "updated_by" character varying NOT NULL,
  "created_at" timestamptz NOT NULL,
  "updated_at" timestamptz NOT NULL,
  PRIMARY KEY ("id")
);

-- create index "roles_created_by" to table: "roles"
CREATE INDEX ("roles_created_by") ON "roles" ("id", "created_by");
-- create index "roles_created_by" to table: "roles"
CREATE INDEX "roles_updated_by" ON "roles" ("updated_by");
-- create index "roles_created_at" to table: "roles"
CREATE INDEX "roles_created_at" ON "roles" ("created_at");
-- create index "roles_updated_at" to table: "roles"
CREATE INDEX "roles_updated_at" ON "roles" ("updated_at");
-- create index "roles_resource_id_name" to table: "roles"
CREATE UNIQUE INDEX "roles_resource_id_name" ON "roles" ("resource_id", "name");

-- +goose Down
-- reverse: create index "roles_resource_id_name" to table: "roles"
DROP INDEX "roles_resource_id_name";
-- reverse: create index "roles_updated_at" to table: "roles"
DROP INDEX "roles_updated_at";
-- reverse: create index "roles_created_at" to table: "roles"
DROP INDEX "roles_created_at";
-- reverse: create index "roles_updated_by" to table: "roles"
DROP INDEX "roles_updated_by";
-- reverse: create index "roles_created_by" to table: "roles"
DROP INDEX "roles_created_by";
-- reverse: create "roles" table
DROP TABLE "roles";
