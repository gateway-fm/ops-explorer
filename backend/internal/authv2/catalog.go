package authv2

import (
	"context"
	"crypto/sha256"
	"encoding/hex"

	"github.com/jackc/pgx/v5"
)

// The structural fingerprint is generated from the exact offline migration,
// independently of role capabilities. OIDs, owners and moving statistics are
// excluded; the runtime role must separately have exactly the intended powers.
const expectedCatalogSHA = "f19698d448edb4bdfc34a339bb2ddd9b0b843599a181235d53f34c1630d20d50"
const catalogSQL = `WITH owned AS (
 SELECT c.* FROM pg_catalog.pg_class c JOIN pg_catalog.pg_namespace n ON n.oid=c.relnamespace WHERE n.nspname='explorer_sso_v2'), tables AS (SELECT * FROM owned WHERE relkind IN ('r','p'))
SELECT pg_catalog.jsonb_build_object(
 'relations',(SELECT pg_catalog.jsonb_agg(pg_catalog.jsonb_build_array(relname,relkind,relpersistence,relispartition,relrowsecurity,relforcerowsecurity,reloptions) ORDER BY relname) FROM owned),
 'columns',(SELECT pg_catalog.jsonb_agg(pg_catalog.jsonb_build_array(c.relname,a.attnum,a.attname,pg_catalog.format_type(a.atttypid,a.atttypmod),a.attnotnull,a.attgenerated,a.attidentity,pg_catalog.pg_get_expr(d.adbin,d.adrelid)) ORDER BY c.relname,a.attnum) FROM tables c JOIN pg_catalog.pg_attribute a ON a.attrelid=c.oid AND a.attnum>0 AND NOT a.attisdropped LEFT JOIN pg_catalog.pg_attrdef d ON d.adrelid=c.oid AND d.adnum=a.attnum),
 'constraints',(SELECT pg_catalog.jsonb_agg(pg_catalog.jsonb_build_array(c.relname,x.conname,x.contype,pg_catalog.pg_get_constraintdef(x.oid,true),x.convalidated,x.condeferrable,x.condeferred) ORDER BY c.relname,x.conname) FROM tables c JOIN pg_catalog.pg_constraint x ON x.conrelid=c.oid),
 'indexes',(SELECT pg_catalog.jsonb_agg(pg_catalog.jsonb_build_array(c.relname,pg_catalog.pg_get_indexdef(i.indexrelid),i.indisvalid,i.indisready) ORDER BY c.relname,pg_catalog.pg_get_indexdef(i.indexrelid)) FROM tables c JOIN pg_catalog.pg_index i ON i.indrelid=c.oid),
 'triggers',(SELECT count(*) FROM pg_catalog.pg_trigger t JOIN tables c ON t.tgrelid=c.oid WHERE NOT t.tgisinternal),
 'rules',(SELECT count(*) FROM pg_catalog.pg_rewrite r JOIN tables c ON r.ev_class=c.oid),
 'policies',(SELECT count(*) FROM pg_catalog.pg_policy p JOIN tables c ON p.polrelid=c.oid),
 'inheritance',(SELECT count(*) FROM pg_catalog.pg_inherits i WHERE i.inhrelid IN(SELECT oid FROM owned) OR i.inhparent IN(SELECT oid FROM owned)),
 'functions',(SELECT count(*) FROM pg_catalog.pg_proc p JOIN pg_catalog.pg_namespace n ON n.oid=p.pronamespace WHERE n.nspname='explorer_sso_v2')
)::text`

const privilegesSQL = `WITH role AS (SELECT * FROM pg_catalog.pg_roles WHERE rolname=current_user), userns AS (
 SELECT n.* FROM pg_catalog.pg_namespace n WHERE n.nspname NOT IN ('pg_catalog','information_schema','pg_toast')
 AND NOT pg_catalog.starts_with(n.nspname,'pg_toast_temp_')
 AND n.oid<>pg_catalog.pg_my_temp_schema() AND NOT pg_catalog.pg_is_other_temp_schema(n.oid)
)
SELECT current_user='explorer_sso_app'
 AND NOT (r.rolsuper OR r.rolinherit OR r.rolcreaterole OR r.rolcreatedb OR r.rolreplication OR r.rolbypassrls)
 AND NOT EXISTS(SELECT 1 FROM pg_catalog.pg_auth_members m WHERE m.member=r.oid)
 AND NOT pg_catalog.has_database_privilege(current_database(),'CREATE')
 AND NOT EXISTS(SELECT 1 FROM userns n WHERE n.nspowner=r.oid OR pg_catalog.has_schema_privilege(n.oid,'CREATE'))
 AND pg_catalog.has_schema_privilege('explorer_sso_v2','USAGE')
 AND NOT EXISTS(SELECT 1 FROM pg_catalog.pg_class c JOIN userns n ON n.oid=c.relnamespace
  WHERE c.relkind IN ('r','p','v','m','f') AND (c.relowner=r.oid OR
   (n.nspname<>'explorer_sso_v2' AND (pg_catalog.has_table_privilege(c.oid,'SELECT,INSERT,UPDATE,DELETE,TRUNCATE,REFERENCES,TRIGGER') OR pg_catalog.has_any_column_privilege(c.oid,'SELECT,INSERT,UPDATE,REFERENCES')))))
 AND NOT EXISTS(SELECT 1 FROM pg_catalog.pg_class c JOIN userns n ON n.oid=c.relnamespace WHERE c.relkind='S' AND (c.relowner=r.oid OR pg_catalog.has_sequence_privilege(c.oid,'SELECT,USAGE,UPDATE')))
 AND NOT EXISTS(SELECT 1 FROM pg_catalog.pg_proc p JOIN userns n ON n.oid=p.pronamespace WHERE p.proowner=r.oid OR pg_catalog.has_function_privilege(p.oid,'EXECUTE'))
 AND pg_catalog.has_table_privilege('explorer_sso_v2.store_state','SELECT')
 AND pg_catalog.has_table_privilege('explorer_sso_v2.store_state','UPDATE')
 AND NOT pg_catalog.has_table_privilege('explorer_sso_v2.store_state','INSERT,DELETE,TRUNCATE,REFERENCES,TRIGGER')
 AND NOT pg_catalog.has_any_column_privilege('explorer_sso_v2.store_state','INSERT,REFERENCES')
 AND pg_catalog.has_table_privilege('explorer_sso_v2.contexts','SELECT')
 AND pg_catalog.has_table_privilege('explorer_sso_v2.contexts','INSERT')
 AND pg_catalog.has_table_privilege('explorer_sso_v2.contexts','UPDATE')
 AND pg_catalog.has_table_privilege('explorer_sso_v2.contexts','DELETE')
 AND NOT pg_catalog.has_table_privilege('explorer_sso_v2.contexts','TRUNCATE,REFERENCES,TRIGGER')
 AND NOT pg_catalog.has_any_column_privilege('explorer_sso_v2.contexts','REFERENCES')
FROM role r`

func checkCatalog(ctx context.Context, tx pgx.Tx) error {
	var raw string
	if tx.QueryRow(ctx, catalogSQL).Scan(&raw) != nil || len(raw) > 32<<10 {
		return ErrUnavailable
	}
	h := sha256.Sum256([]byte(raw))
	if hex.EncodeToString(h[:]) != expectedCatalogSHA {
		return ErrUnavailable
	}
	var valid bool
	if tx.QueryRow(ctx, privilegesSQL).Scan(&valid) != nil || !valid {
		return ErrUnavailable
	}
	return nil
}
