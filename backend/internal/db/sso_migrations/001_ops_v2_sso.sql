-- Operational app sessions only; migration owner runs this offline.
-- Role explorer_sso_app is externally provisioned NOINHERIT before migration.
CREATE SCHEMA explorer_sso_v2;
REVOKE ALL ON SCHEMA explorer_sso_v2 FROM PUBLIC;
CREATE TABLE explorer_sso_v2.store_state (
 id smallint PRIMARY KEY CHECK (id=1),
 config_digest bytea NOT NULL CHECK (octet_length(config_digest)=32),
 recovery_epoch uuid NOT NULL CHECK (recovery_epoch<>'00000000-0000-0000-0000-000000000000'),
 ready boolean NOT NULL DEFAULT false,
 max_contexts integer NOT NULL CHECK (max_contexts BETWEEN 1 AND 200000),
 max_cipher_bytes bigint NOT NULL CHECK (max_cipher_bytes BETWEEN 1 AND 1073741824),
 max_metadata_bytes bigint NOT NULL CHECK (max_metadata_bytes BETWEEN 2048 AND 536870912),
 context_count integer NOT NULL DEFAULT 0 CHECK (context_count BETWEEN 0 AND max_contexts),
 cipher_bytes bigint NOT NULL DEFAULT 0 CHECK (cipher_bytes BETWEEN 0 AND max_cipher_bytes),
 CHECK (context_count::bigint*2048<=max_metadata_bytes)
);
CREATE TABLE explorer_sso_v2.contexts (
 browser_hash bytea PRIMARY KEY CHECK (octet_length(browser_hash)=32),
 config_digest bytea NOT NULL CHECK (octet_length(config_digest)=32),
 recovery_epoch uuid NOT NULL CHECK (recovery_epoch<>'00000000-0000-0000-0000-000000000000'),
 created_at bigint NOT NULL CHECK (created_at BETWEEN 0 AND 253402300469),
 last_change bigint NOT NULL CHECK (last_change BETWEEN created_at AND 253402300469),
 retain_until bigint NOT NULL CHECK (retain_until BETWEEN last_change AND last_change+330),
 pending_revision uuid NOT NULL CHECK (pending_revision<>'00000000-0000-0000-0000-000000000000'),
 pending_hash bytea,
 pending_issued_at bigint,
 pending_expires_at bigint,
 active_auth_hash bytea,
 activation uuid,
 activated_at bigint,
 token_key_id varchar(32),
 token_cipher bytea,
 original_token_expiry bigint,
 CHECK ((pending_hash IS NULL AND pending_issued_at IS NULL AND pending_expires_at IS NULL) OR
        (pending_hash IS NOT NULL AND octet_length(pending_hash)=32 AND pending_issued_at IS NOT NULL AND pending_expires_at IS NOT NULL AND
         pending_issued_at BETWEEN 0 AND last_change AND pending_expires_at>pending_issued_at AND
         pending_expires_at<=pending_issued_at+300 AND pending_expires_at<=retain_until)),
 CHECK ((active_auth_hash IS NULL AND activation IS NULL AND activated_at IS NULL AND token_key_id IS NULL AND token_cipher IS NULL AND original_token_expiry IS NULL) OR
        (active_auth_hash IS NOT NULL AND octet_length(active_auth_hash)=32 AND activation IS NOT NULL AND
         activation<>'00000000-0000-0000-0000-000000000000' AND activated_at IS NOT NULL AND activated_at BETWEEN 0 AND last_change AND
         token_key_id IS NOT NULL AND token_key_id ~ '^[a-z][a-z0-9_-]{0,31}$' AND token_cipher IS NOT NULL AND octet_length(token_cipher) BETWEEN 29 AND 16412 AND
         original_token_expiry IS NOT NULL AND original_token_expiry>activated_at AND original_token_expiry<=activated_at+330 AND original_token_expiry<=retain_until))
);
CREATE INDEX contexts_retention ON explorer_sso_v2.contexts (retain_until,browser_hash);
REVOKE ALL ON ALL TABLES IN SCHEMA explorer_sso_v2 FROM PUBLIC;
GRANT USAGE ON SCHEMA explorer_sso_v2 TO explorer_sso_app;
GRANT SELECT,UPDATE ON explorer_sso_v2.store_state TO explorer_sso_app;
GRANT SELECT,INSERT,UPDATE,DELETE ON explorer_sso_v2.contexts TO explorer_sso_app;

---- create above / drop below ----

DROP SCHEMA explorer_sso_v2 CASCADE;
