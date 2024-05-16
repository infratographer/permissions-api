-- +goose Up

-- create "zedtokens" table
CREATE TABLE "zedtokens" (
  "resource_id" character varying NOT NULL,
  "zedtoken" character varying NOT NULL,
  "created_at" timestamptz NOT NULL,
  "expires_at" timestamptz NOT NULL,
  PRIMARY KEY ("resource_id")
) WITH (ttl_expiration_expression = 'expires_at', ttl_job_cron = '0 */4 * * *');

-- +goose Down

-- drop "zedtokens" table
DROP TABLE "zedtokens";
