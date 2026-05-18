package auth

import (
	"crypto/rand"
	"encoding/base64"
	"sync"
	"time"
)

type LoginState struct {
	Mode, Redirect, CLIRedirect, CodeChallenge string
	ExpiresAt                                  time.Time
	GoogleUser                                 GoogleUser
	UserID                                     string
}

type LoginStateStore interface {
	PutState(id string, st LoginState) error
	TakeState(id string) (LoginState, bool)
	PutCode(code string, st LoginState) error
	TakeCode(code string) (LoginState, bool)
}

type StateStore struct {
	mu       sync.Mutex
	states   map[string]LoginState
	cliCodes map[string]LoginState
}

func NewStateStore() *StateStore {
	return &StateStore{states: map[string]LoginState{}, cliCodes: map[string]LoginState{}}
}
func (s *StateStore) PutState(id string, st LoginState) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.states[id] = st
	return nil
}
func (s *StateStore) TakeState(id string) (LoginState, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	st, ok := s.states[id]
	delete(s.states, id)
	if !ok || time.Now().After(st.ExpiresAt) {
		return LoginState{}, false
	}
	return st, true
}
func (s *StateStore) PutCode(code string, st LoginState) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	st.ExpiresAt = time.Now().Add(5 * time.Minute)
	s.cliCodes[code] = st
	return nil
}
func (s *StateStore) TakeCode(code string) (LoginState, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	st, ok := s.cliCodes[code]
	delete(s.cliCodes, code)
	if !ok || time.Now().After(st.ExpiresAt) {
		return LoginState{}, false
	}
	return st, true
}
func RandomURLToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}
