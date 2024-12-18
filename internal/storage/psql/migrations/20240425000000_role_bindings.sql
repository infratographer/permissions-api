-- +goose NO TRANSACTION
-- +goose Up

-- create "rolebindings" table
CREATE TABLE "rolebindings" (
  "id" character varying NOT NULL,
  "resource_id" character varying NOT NULL,
  "created_by" character varying NOT NULL,
  "updated_by" character varying NOT NULL,
  "created_at" timestamptz NOT NULL,
  "updated_at" timestamptz NOT NULL,
  PRIMARY KEY ("id")
);

-- create index "rolebindings_created_by" to table: "rolebindings"
CREATE INDEX "rolebindings_created_by" ON "rolebindings" ("created_by");
-- create index "rolebindings_created_by" to table: "rolebindings"
CREATE INDEX "rolebindings_updated_by" ON "rolebindings" ("updated_by");
-- create index "rolebindings_created_at" to table: "rolebindings"
CREATE INDEX "rolebindings_created_at" ON "rolebindings" ("created_at");
-- create index "rolebindings_updated_at" to table: "rolebindings"
CREATE INDEX "rolebindings_updated_at" ON "rolebindings" ("updated_at");

-- +goose Down
-- reverse: create index "rolebindings_updated_at" to table: "rolebindings"
DROP INDEX "rolebindings_updated_at";
-- reverse: create index "rolebindings_created_at" to table: "rolebindings"
DROP INDEX "rolebindings_created_at";
-- reverse: create index "rolebindings_updated_by" to table: "rolebindings"
DROP INDEX "rolebindings_updated_by";
-- reverse: create index "rolebindings_created_by" to table: "rolebindings"
DROP INDEX "rolebindings_created_by";
-- reverse: create "rolebindings" table
DROP TABLE "rolebindings";
