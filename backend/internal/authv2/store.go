package authv2

import (
	"bytes"
	"context"
	"errors"
	"sync/atomic"
	"time"

	uuidlib "github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type store struct {
	source *Source
	pool   *pgxpool.Pool
	clock  Clock
	closed atomic.Bool
}
type contextRow struct {
	browser, config               []byte
	epoch                         string
	created, changed, retain      int64
	revision                      string
	pendingHash                   []byte
	pendingIssued, pendingExpires *int64
	auth                          []byte
	activation                    *string
	activated                     *int64
	key                           *string
	cipher                        []byte
	expires                       *int64
}

const rowColumns = `browser_hash,config_digest,recovery_epoch::text,created_at,last_change,retain_until,pending_revision::text,pending_hash,pending_issued_at,pending_expires_at,active_auth_hash,activation::text,activated_at,token_key_id,token_cipher,original_token_expiry`

func readRow(row pgx.Row) (*contextRow, error) {
	r := &contextRow{}
	err := row.Scan(&r.browser, &r.config, &r.epoch, &r.created, &r.changed, &r.retain, &r.revision, &r.pendingHash, &r.pendingIssued, &r.pendingExpires, &r.auth, &r.activation, &r.activated, &r.key, &r.cipher, &r.expires)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, ErrUnavailable
	}
	return r, nil
}
func (r *contextRow) valid(s *Source) bool {
	if r == nil || len(r.browser) != 32 || !bytes.Equal(r.config, s.digest[:]) || r.epoch != s.spec.RecoveryEpoch || r.created < 0 || r.changed < r.created || r.changed > MaxTime-330 || r.retain < r.changed || r.retain > r.changed+330 || !uuidString(r.revision) {
		return false
	}
	if r.pendingHash == nil {
		if r.pendingIssued != nil || r.pendingExpires != nil {
			return false
		}
	} else if len(r.pendingHash) != 32 || r.pendingIssued == nil || r.pendingExpires == nil || *r.pendingIssued < 0 || *r.pendingIssued > r.changed || *r.pendingExpires <= *r.pendingIssued || *r.pendingExpires > *r.pendingIssued+300 || *r.pendingExpires > r.retain {
		return false
	}
	if r.auth == nil {
		return r.activation == nil && r.activated == nil && r.key == nil && r.cipher == nil && r.expires == nil
	}
	return len(r.auth) == 32 && r.activation != nil && uuidString(*r.activation) && r.activated != nil && *r.activated >= 0 && *r.activated <= r.changed && r.key != nil && *r.key == s.spec.TokenKey.ID && len(r.cipher) >= 29 && len(r.cipher) <= MaxCipherBytes && r.expires != nil && *r.expires > *r.activated && *r.expires <= *r.activated+330 && *r.expires <= r.retain
}
func uuidString(v string) bool { return uuid(v) }
func (r *contextRow) pending() (pendingCapture, bool) {
	if r == nil || r.pendingHash == nil {
		return pendingCapture{}, false
	}
	p := pendingCapture{revision: r.revision, issued: *r.pendingIssued, expires: *r.pendingExpires}
	copy(p.browser[:], r.browser)
	copy(p.hash[:], r.pendingHash)
	return p, true
}
func (r *contextRow) active() (activeCapture, bool) {
	if r == nil || r.auth == nil {
		return activeCapture{}, false
	}
	a := activeCapture{activation: *r.activation, activated: *r.activated, expires: *r.expires, key: *r.key, cipher: append([]byte(nil), r.cipher...)}
	copy(a.browser[:], r.browser)
	copy(a.auth[:], r.auth)
	return a, true
}
func openStore(ctx context.Context, s *Source, dsn string, clock Clock) (*store, error) {
	if ctx == nil || s == nil || clock == nil || dsn == "" {
		return nil, ErrInvalid
	}
	cfg, e := pgxpool.ParseConfig(dsn)
	if e != nil || cfg.ConnConfig.User != AppRole {
		return nil, ErrInvalid
	}
	cfg.MaxConns = 8
	cfg.MinConns = 0
	cfg.ConnConfig.ConnectTimeout = 5 * time.Second
	cfg.MaxConnLifetime = 30 * time.Minute
	cfg.MaxConnIdleTime = 5 * time.Minute
	p, e := pgxpool.NewWithConfig(ctx, cfg)
	if e != nil {
		return nil, ErrUnavailable
	}
	out := &store{s, p, clock, atomic.Bool{}}
	_, e = out.read(ctx, [32]byte{})
	if e != nil {
		p.Close()
		return nil, e
	}
	return out, nil
}
func (s *store) close() {
	if s != nil && s.closed.CompareAndSwap(false, true) {
		s.pool.Close()
	}
}
func (s *store) now() (int64, error) {
	n := s.clock()
	if n < 0 || n > MaxTime-330 {
		return 0, ErrUnavailable
	}
	return n, nil
}

type quota struct {
	rows   int
	cipher int64
}

func (s *store) begin(ctx context.Context, write bool) (pgx.Tx, context.Context, context.CancelFunc, error) {
	if ctx == nil || s == nil || s.pool == nil || s.closed.Load() {
		return nil, ctx, func() {}, ErrUnavailable
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	mode := pgx.ReadOnly
	if write {
		mode = pgx.ReadWrite
	}
	tx, e := s.pool.BeginTx(ctx, pgx.TxOptions{AccessMode: mode})
	if e != nil {
		cancel()
		return nil, ctx, func() {}, ErrUnavailable
	}
	if _, e = tx.Exec(ctx, `SET LOCAL statement_timeout='2s'; SET LOCAL lock_timeout='500ms'; SET LOCAL synchronous_commit=on; SET LOCAL search_path=pg_catalog`); e != nil {
		_ = tx.Rollback(ctx)
		cancel()
		return nil, ctx, func() {}, ErrUnavailable
	}
	if e = checkCatalog(ctx, tx); e != nil {
		_ = tx.Rollback(ctx)
		cancel()
		return nil, ctx, func() {}, e
	}
	return tx, ctx, cancel, nil
}
func (s *store) state(ctx context.Context, tx pgx.Tx, lock bool) (quota, error) {
	q := `SELECT config_digest,recovery_epoch::text,ready,max_contexts,max_cipher_bytes,max_metadata_bytes,context_count,cipher_bytes FROM explorer_sso_v2.store_state WHERE id=1`
	if lock {
		q += ` FOR UPDATE`
	}
	var digest []byte
	var epoch string
	var ready bool
	var limits Limits
	var quota quota
	if tx.QueryRow(ctx, q).Scan(&digest, &epoch, &ready, &limits.MaxContexts, &limits.MaxCipherBytes, &limits.MaxMetadataBytes, &quota.rows, &quota.cipher) != nil || !bytes.Equal(digest, s.source.digest[:]) || epoch != s.source.spec.RecoveryEpoch || !ready || limits != s.source.spec.Limits || quota.rows < 0 || quota.rows > limits.MaxContexts || quota.cipher < 0 || quota.cipher > limits.MaxCipherBytes || int64(quota.rows)*2048 > limits.MaxMetadataBytes {
		return quota, ErrUnavailable
	}
	return quota, nil
}
func (s *store) read(ctx context.Context, browser [32]byte) (*contextRow, error) {
	tx, ctx, cancel, e := s.begin(ctx, false)
	if e != nil {
		return nil, e
	}
	defer cancel()
	defer tx.Rollback(ctx)
	// One MVCC statement pairs readiness/epoch and the indexed context. Separate
	// READ COMMITTED statements could combine a prior ready state with later data.
	r := &contextRow{}
	var digest []byte
	var epoch string
	var ready bool
	var limits Limits
	var quota quota
	e = tx.QueryRow(ctx, `SELECT s.config_digest,s.recovery_epoch::text,s.ready,s.max_contexts,s.max_cipher_bytes,s.max_metadata_bytes,s.context_count,s.cipher_bytes,
 c.browser_hash,c.config_digest,COALESCE(c.recovery_epoch::text,''),COALESCE(c.created_at,0),COALESCE(c.last_change,0),COALESCE(c.retain_until,0),COALESCE(c.pending_revision::text,''),c.pending_hash,c.pending_issued_at,c.pending_expires_at,c.active_auth_hash,c.activation::text,c.activated_at,c.token_key_id,c.token_cipher,c.original_token_expiry
 FROM explorer_sso_v2.store_state s LEFT JOIN explorer_sso_v2.contexts c ON c.browser_hash=$1 WHERE s.id=1`, browser[:]).Scan(&digest, &epoch, &ready, &limits.MaxContexts, &limits.MaxCipherBytes, &limits.MaxMetadataBytes, &quota.rows, &quota.cipher, &r.browser, &r.config, &r.epoch, &r.created, &r.changed, &r.retain, &r.revision, &r.pendingHash, &r.pendingIssued, &r.pendingExpires, &r.auth, &r.activation, &r.activated, &r.key, &r.cipher, &r.expires)
	if e != nil || !bytes.Equal(digest, s.source.digest[:]) || epoch != s.source.spec.RecoveryEpoch || !ready || limits != s.source.spec.Limits || quota.rows < 0 || quota.rows > limits.MaxContexts || quota.cipher < 0 || quota.cipher > limits.MaxCipherBytes || int64(quota.rows)*2048 > limits.MaxMetadataBytes {
		return nil, ErrUnavailable
	}
	if r.browser == nil {
		r = nil
	} else if !r.valid(s.source) {
		return nil, ErrUnavailable
	}
	if e = tx.Commit(ctx); e != nil {
		return nil, ErrUnknownCommit
	}
	return r, nil
}
func (s *store) mutate(ctx context.Context, browser [32]byte, op func(context.Context, pgx.Tx, *contextRow, int64) (int, int64, error)) error {
	tx, ctx, cancel, e := s.begin(ctx, true)
	if e != nil {
		return e
	}
	defer cancel()
	defer tx.Rollback(ctx)
	q, e := s.state(ctx, tx, true)
	if e != nil {
		return e
	}
	r, e := readRow(tx.QueryRow(ctx, `SELECT `+rowColumns+` FROM explorer_sso_v2.contexts WHERE browser_hash=$1 FOR UPDATE`, browser[:]))
	if e != nil {
		return e
	}
	if r != nil && !r.valid(s.source) {
		return ErrUnavailable
	}
	now, e := s.now()
	if e != nil || r != nil && now < r.changed {
		return ErrUnavailable
	}
	rows, cipher, e := op(ctx, tx, r, now)
	if e != nil {
		return e
	}
	q.rows += rows
	q.cipher += cipher
	l := s.source.spec.Limits
	if q.rows < 0 || q.rows > l.MaxContexts || q.cipher < 0 || q.cipher > l.MaxCipherBytes || int64(q.rows)*2048 > l.MaxMetadataBytes {
		return ErrUnavailable
	}
	tag, e := tx.Exec(ctx, `UPDATE explorer_sso_v2.store_state SET context_count=$1,cipher_bytes=$2 WHERE id=1 AND config_digest=$3 AND recovery_epoch=$4 AND ready`, q.rows, q.cipher, s.source.digest[:], s.source.spec.RecoveryEpoch)
	if e != nil || tag.RowsAffected() != 1 {
		return ErrUnavailable
	}
	if tx.Commit(ctx) != nil {
		return ErrUnknownCommit
	}
	return nil
}
func (s *store) start(ctx context.Context, p pendingCapture) error {
	return s.startGuarded(ctx, p, nil, false)
}
func (s *store) startGuarded(ctx context.Context, p pendingCapture, expected *activeCapture, renew bool) error {
	return s.mutate(ctx, p.browser, func(ctx context.Context, tx pgx.Tx, r *contextRow, now int64) (int, int64, error) {
		if renew {
			current, ok := r.active()
			if expected != nil {
				if expected.expires <= now {
					return 0, 0, ErrUnauthenticated
				}
				if !ok || !sameActive(current, *expected) {
					return 0, 0, ErrChanged
				}
			} else if ok && current.expires > now {
				return 0, 0, ErrChanged
			}
		}
		if p.issued < 0 || p.issued > now || p.expires <= now || p.expires > p.issued+300 {
			return 0, 0, ErrInvalid
		}
		if r == nil {
			tag, e := tx.Exec(ctx, `INSERT INTO explorer_sso_v2.contexts(browser_hash,config_digest,recovery_epoch,created_at,last_change,retain_until,pending_revision,pending_hash,pending_issued_at,pending_expires_at) VALUES($1,$2,$3,$4,$4,$4::bigint+300,$5,$6,$7,$8)`, p.browser[:], s.source.digest[:], s.source.spec.RecoveryEpoch, now, p.revision, p.hash[:], p.issued, p.expires)
			if e != nil || tag.RowsAffected() != 1 {
				return 0, 0, ErrUnavailable
			}
			return 1, 0, nil
		}
		tag, e := tx.Exec(ctx, `UPDATE explorer_sso_v2.contexts SET pending_revision=$1,pending_hash=$2,pending_issued_at=$3,pending_expires_at=$4,last_change=$5,retain_until=GREATEST($5::bigint+300,$4,original_token_expiry) WHERE browser_hash=$6 AND config_digest=$7 AND recovery_epoch=$8 AND pending_revision=$9`, p.revision, p.hash[:], p.issued, p.expires, now, p.browser[:], s.source.digest[:], s.source.spec.RecoveryEpoch, r.revision)
		if e != nil || tag.RowsAffected() != 1 {
			return 0, 0, ErrChanged
		}
		return 0, 0, nil
	})
}
func samePending(a, b pendingCapture) bool {
	return a.browser == b.browser && a.revision == b.revision && a.hash == b.hash && a.issued == b.issued && a.expires == b.expires
}
func sameActive(a, b activeCapture) bool {
	return a.browser == b.browser && a.auth == b.auth && a.activation == b.activation && a.activated == b.activated && a.expires == b.expires && a.key == b.key && bytes.Equal(a.cipher, b.cipher)
}
func (s *store) activate(ctx context.Context, p pendingCapture, a activeCapture) error {
	return s.mutate(ctx, p.browser, func(ctx context.Context, tx pgx.Tx, r *contextRow, now int64) (int, int64, error) {
		current, ok := r.pending()
		if !ok || !samePending(current, p) || p.expires <= now {
			return 0, 0, ErrChanged
		}
		if a.browser != p.browser || a.expires <= now || a.activated > now || a.expires > a.activated+330 {
			return 0, 0, ErrUnauthenticated
		}
		tag, e := tx.Exec(ctx, `UPDATE explorer_sso_v2.contexts SET pending_revision=$1,pending_hash=NULL,pending_issued_at=NULL,pending_expires_at=NULL,active_auth_hash=$2,activation=$3,activated_at=$4,token_key_id=$5,token_cipher=$6,original_token_expiry=$7,last_change=$8,retain_until=GREATEST($8::bigint+300,$7) WHERE browser_hash=$9 AND config_digest=$10 AND recovery_epoch=$11 AND pending_revision=$12 AND pending_hash=$13 AND pending_issued_at=$14 AND pending_expires_at=$15`, uuidlib.NewString(), a.auth[:], a.activation, a.activated, a.key, a.cipher, a.expires, now, p.browser[:], s.source.digest[:], s.source.spec.RecoveryEpoch, p.revision, p.hash[:], p.issued, p.expires)
		if e != nil || tag.RowsAffected() != 1 {
			return 0, 0, ErrChanged
		}
		return 0, int64(len(a.cipher) - len(r.cipher)), nil
	})
}
func (s *store) logout(ctx context.Context, a activeCapture) error {
	return s.mutate(ctx, a.browser, func(ctx context.Context, tx pgx.Tx, r *contextRow, now int64) (int, int64, error) {
		if a.expires <= now {
			return 0, 0, ErrUnauthenticated
		}
		current, ok := r.active()
		if !ok || !sameActive(current, a) {
			return 0, 0, ErrChanged
		}
		tag, e := tx.Exec(ctx, `UPDATE explorer_sso_v2.contexts SET pending_revision=$1,pending_hash=NULL,pending_issued_at=NULL,pending_expires_at=NULL,active_auth_hash=NULL,activation=NULL,activated_at=NULL,token_key_id=NULL,token_cipher=NULL,original_token_expiry=NULL,last_change=$2,retain_until=$2::bigint+300 WHERE browser_hash=$3 AND config_digest=$4 AND recovery_epoch=$5 AND activation=$6 AND active_auth_hash=$7 AND token_cipher=$8`, uuidlib.NewString(), now, a.browser[:], s.source.digest[:], s.source.spec.RecoveryEpoch, a.activation, a.auth[:], a.cipher)
		if e != nil || tag.RowsAffected() != 1 {
			return 0, 0, ErrChanged
		}
		return 0, -int64(len(r.cipher)), nil
	})
}
