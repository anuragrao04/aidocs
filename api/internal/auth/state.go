package auth

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"sync"
	"time"
)

// defaultStateTTL is applied by PutState when the caller leaves ExpiresAt unset
// (zero), so a forgotten expiry does not make every state instantly expired.
const defaultStateTTL = 10 * time.Minute

// codeTTL is the lifetime of a CLI exchange code.
const codeTTL = 5 * time.Minute

type LoginState struct {
	Mode, Redirect, CLIRedirect, CodeChallenge string
	ExpiresAt                                  time.Time
	GoogleUser                                 GoogleUser
	UserID                                     string
}

type LoginStateStore interface {
	PutState(ctx context.Context, id string, st LoginState) error
	TakeState(ctx context.Context, id string) (LoginState, bool, error)
	PutCode(ctx context.Context, code string, st LoginState) error
	TakeCode(ctx context.Context, code string) (LoginState, bool, error)
}

type StateStore struct {
	mu       sync.Mutex
	states   map[string]LoginState
	cliCodes map[string]LoginState
}

func NewStateStore() *StateStore {
	return &StateStore{states: map[string]LoginState{}, cliCodes: map[string]LoginState{}}
}
func (s *StateStore) PutState(ctx context.Context, id string, st LoginState) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if st.ExpiresAt.IsZero() {
		st.ExpiresAt = time.Now().Add(defaultStateTTL)
	}
	s.states[id] = st
	return nil
}
func (s *StateStore) TakeState(ctx context.Context, id string) (LoginState, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	st, ok := s.states[id]
	delete(s.states, id)
	if !ok || time.Now().After(st.ExpiresAt) {
		return LoginState{}, false, nil
	}
	return st, true, nil
}
func (s *StateStore) PutCode(ctx context.Context, code string, st LoginState) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	st.ExpiresAt = time.Now().Add(codeTTL)
	s.cliCodes[code] = st
	return nil
}
func (s *StateStore) TakeCode(ctx context.Context, code string) (LoginState, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	st, ok := s.cliCodes[code]
	delete(s.cliCodes, code)
	if !ok || time.Now().After(st.ExpiresAt) {
		return LoginState{}, false, nil
	}
	return st, true, nil
}
func RandomURLToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}
