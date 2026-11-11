// Package authv2admin is the offline migration-owner tool. The serving command
// never imports it or accepts its credential. Recovery requires drained traffic.
package authv2admin

import (
	"context"
	"errors"
	"explorer/internal/authv2"
	"explorer/internal/db"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/tern/v2/migrate"
	"time"
)

var ErrInvalid = errors.New("invalid offline SSO administration")

func Run(parent context.Context, dsn string, source *authv2.Source, action string, drained bool) error {
	if parent == nil || source == nil || dsn == "" || action != "migrate" && action != "initialize" && action != "drained-recover" || action == "drained-recover" && !drained {
		return ErrInvalid
	}
	ctx, cancel := context.WithTimeout(parent, 30*time.Second)
	defer cancel()
	conn, e := pgx.Connect(ctx, dsn)
	if e != nil {
		return e
	}
	defer conn.Close(ctx)
	var user string
	if conn.QueryRow(ctx, `SELECT current_user`).Scan(&user) != nil || user == authv2.AppRole {
		return ErrInvalid
	}
	if action == "migrate" {
		files, e := db.SSOMigrationFiles()
		if e != nil {
			return e
		}
		m, e := migrate.NewMigrator(ctx, conn, "public.explorer_sso_migration_version")
		if e != nil {
			return e
		}
		if e = m.LoadMigrations(files); e != nil {
			return e
		}
		return m.Migrate(ctx)
	}
	tx, e := conn.Begin(ctx)
	if e != nil {
		return e
	}
	defer tx.Rollback(ctx)
	if _, e = tx.Exec(ctx, `SET LOCAL statement_timeout='20s'; SET LOCAL lock_timeout='2s'; SET LOCAL synchronous_commit=on; SET LOCAL search_path=pg_catalog; LOCK TABLE explorer_sso_v2.store_state IN ACCESS EXCLUSIVE MODE; LOCK TABLE explorer_sso_v2.contexts IN ACCESS EXCLUSIVE MODE`); e != nil {
		return e
	}
	m := source.StorageMetadata()
	if action == "initialize" {
		var n int
		if tx.QueryRow(ctx, `SELECT (SELECT count(*) FROM explorer_sso_v2.store_state)+(SELECT count(*) FROM explorer_sso_v2.contexts)`).Scan(&n) != nil || n != 0 {
			return ErrInvalid
		}
		_, e = tx.Exec(ctx, `INSERT INTO explorer_sso_v2.store_state(id,config_digest,recovery_epoch,ready,max_contexts,max_cipher_bytes,max_metadata_bytes,context_count,cipher_bytes) VALUES(1,$1,$2,true,$3,$4,$5,0,0)`, m.ConfigDigest[:], m.RecoveryEpoch, m.Limits.MaxContexts, m.Limits.MaxCipherBytes, m.Limits.MaxMetadataBytes)
	} else {
		if _, e = tx.Exec(ctx, `DELETE FROM explorer_sso_v2.contexts; DELETE FROM explorer_sso_v2.store_state`); e != nil {
			return e
		}
		_, e = tx.Exec(ctx, `INSERT INTO explorer_sso_v2.store_state(id,config_digest,recovery_epoch,ready,max_contexts,max_cipher_bytes,max_metadata_bytes,context_count,cipher_bytes) VALUES(1,$1,$2,true,$3,$4,$5,0,0)`, m.ConfigDigest[:], m.RecoveryEpoch, m.Limits.MaxContexts, m.Limits.MaxCipherBytes, m.Limits.MaxMetadataBytes)
	}
	if e != nil {
		return e
	}
	return tx.Commit(ctx)
}
