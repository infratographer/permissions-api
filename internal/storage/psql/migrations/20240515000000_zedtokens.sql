-- +goose NO TRANSACTION
-- +goose Up

-- create "zedtokens" table
CREATE TABLE "zedtokens" (
  "resource_id" character varying NOT NULL,
  "zedtoken" character varying NOT NULL,
  "created_at" timestamptz NOT NULL,
  "expires_at" timestamptz NOT NULL,
  PRIMARY KEY ("resource_id")
) TTL INTERVAL '1 day' ON "expires_at";

-- +goose Down

-- drop "zedtokens" table
DROP TABLE "zedtokens";
