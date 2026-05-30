package auth

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	oauthStateKind = "state"
	oauthCodeKind  = "cli_code"
)

type PostgresStateStore struct{ db *pgxpool.Pool }

func NewPostgresStateStore(db *pgxpool.Pool) *PostgresStateStore {
	return &PostgresStateStore{db: db}
}

func (s *PostgresStateStore) PutState(ctx context.Context, id string, st LoginState) error {
	if st.ExpiresAt.IsZero() {
		st.ExpiresAt = time.Now().Add(10 * time.Minute)
	}
	return s.put(ctx, oauthStateKind, id, st)
}

func (s *PostgresStateStore) TakeState(ctx context.Context, id string) (LoginState, bool, error) {
	return s.take(ctx, oauthStateKind, id)
}

func (s *PostgresStateStore) PutCode(ctx context.Context, code string, st LoginState) error {
	st.ExpiresAt = time.Now().Add(5 * time.Minute)
	return s.put(ctx, oauthCodeKind, code, st)
}

func (s *PostgresStateStore) TakeCode(ctx context.Context, code string) (LoginState, bool, error) {
	return s.take(ctx, oauthCodeKind, code)
}

func (s *PostgresStateStore) put(ctx context.Context, kind, id string, st LoginState) error {
	b, err := json.Marshal(st)
	if err != nil {
		return err
	}
	_, err = s.db.Exec(ctx, `
		INSERT INTO oauth_states(id, kind, state_json, expires_at)
		VALUES($1, $2, $3, $4)
		ON CONFLICT(kind, id) DO UPDATE SET
		  state_json=EXCLUDED.state_json,
		  expires_at=EXCLUDED.expires_at,
		  created_at=now()
	`, id, kind, b, st.ExpiresAt)
	return err
}

func (s *PostgresStateStore) take(ctx context.Context, kind, id string) (LoginState, bool, error) {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return LoginState{}, false, err
	}
	defer tx.Rollback(ctx)

	var raw []byte
	var expiresAt time.Time
	err = tx.QueryRow(ctx, `
		DELETE FROM oauth_states
		WHERE kind=$1 AND id=$2
		RETURNING state_json, expires_at
	`, kind, id).Scan(&raw, &expiresAt)
	if errors.Is(err, pgx.ErrNoRows) {
		// Genuinely not found (or already consumed): not an error.
		return LoginState{}, false, nil
	}
	if err != nil {
		// A real DB error must be distinguishable from "not found" so callers
		// can alert on OAuth/CLI exchange failures.
		return LoginState{}, false, err
	}
	if time.Now().After(expiresAt) {
		return LoginState{}, false, nil
	}
	var st LoginState
	if err := json.Unmarshal(raw, &st); err != nil {
		return LoginState{}, false, err
	}
	if err := tx.Commit(ctx); err != nil {
		return LoginState{}, false, err
	}
	return st, true, nil
}

func (s *PostgresStateStore) CleanupExpired(ctx context.Context) error {
	_, err := s.db.Exec(ctx, `DELETE FROM oauth_states WHERE expires_at < now()`)
	return err
}

var _ LoginStateStore = (*PostgresStateStore)(nil)
