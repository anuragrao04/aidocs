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

func (s *PostgresStateStore) PutState(id string, st LoginState) error {
	return s.put(oauthStateKind, id, st)
}

func (s *PostgresStateStore) TakeState(id string) (LoginState, bool) {
	return s.take(oauthStateKind, id)
}

func (s *PostgresStateStore) PutCode(code string, st LoginState) error {
	st.ExpiresAt = time.Now().Add(5 * time.Minute)
	return s.put(oauthCodeKind, code, st)
}

func (s *PostgresStateStore) TakeCode(code string) (LoginState, bool) {
	return s.take(oauthCodeKind, code)
}

func (s *PostgresStateStore) put(kind, id string, st LoginState) error {
	b, err := json.Marshal(st)
	if err != nil {
		return err
	}
	_, err = s.db.Exec(context.Background(), `
		INSERT INTO oauth_states(id, kind, state_json, expires_at)
		VALUES($1, $2, $3, $4)
		ON CONFLICT(kind, id) DO UPDATE SET
		  state_json=EXCLUDED.state_json,
		  expires_at=EXCLUDED.expires_at,
		  created_at=now()
	`, id, kind, b, st.ExpiresAt)
	return err
}

func (s *PostgresStateStore) take(kind, id string) (LoginState, bool) {
	tx, err := s.db.Begin(context.Background())
	if err != nil {
		return LoginState{}, false
	}
	defer tx.Rollback(context.Background())

	var raw []byte
	var expiresAt time.Time
	err = tx.QueryRow(context.Background(), `
		DELETE FROM oauth_states
		WHERE kind=$1 AND id=$2
		RETURNING state_json, expires_at
	`, kind, id).Scan(&raw, &expiresAt)
	if errors.Is(err, pgx.ErrNoRows) || err != nil || time.Now().After(expiresAt) {
		return LoginState{}, false
	}
	var st LoginState
	if err := json.Unmarshal(raw, &st); err != nil {
		return LoginState{}, false
	}
	if err := tx.Commit(context.Background()); err != nil {
		return LoginState{}, false
	}
	return st, true
}

func (s *PostgresStateStore) CleanupExpired(ctx context.Context) error {
	_, err := s.db.Exec(ctx, `DELETE FROM oauth_states WHERE expires_at < now()`)
	return err
}

var _ LoginStateStore = (*PostgresStateStore)(nil)
