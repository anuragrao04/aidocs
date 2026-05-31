package repo

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/anuragrao/aidocs/api/internal/auth"
)

type Role string

const (
	RoleNone      Role = ""
	RoleViewer    Role = "viewer"
	RoleCommenter Role = "commenter"
	RoleEditor    Role = "editor"
	RoleOwner     Role = "owner"
)

var roleRank = map[Role]int{RoleViewer: 1, RoleCommenter: 2, RoleEditor: 3, RoleOwner: 4}

// MaxRole returns whichever role grants more access. RoleNone ("") ranks below
// every real role, so MaxRole(RoleNone, x) == x.
func MaxRole(a, b Role) Role {
	if roleRank[b] > roleRank[a] {
		return b
	}
	return a
}

// ErrNotFound wraps auth.ErrNotFound so that packages which cannot import repo
// (to avoid an import cycle) can still detect not-found via errors.Is.
var ErrNotFound = fmt.Errorf("not found: %w", auth.ErrNotFound)
var ErrConflict = errors.New("conflict")

// Ownership-transfer state errors, shared by all repository implementations so
// handlers can map them to responses without matching on message text.
var (
	ErrNotTransferTarget  = errors.New("not transfer target")
	ErrTransferNotPending = errors.New("transfer not pending")
)

// ErrVersionConflict indicates an optimistic-concurrency failure when creating
// a new version: the supplied base_version_id is stale. The concrete error
// returned is a *VersionConflictError carrying the latest version id; callers
// can match it with errors.Is(err, ErrVersionConflict) or extract details with
// errors.As(&VersionConflictError{}).
var ErrVersionConflict = errors.New("version_conflict")

// VersionConflictError carries the current/latest version id alongside the
// sentinel ErrVersionConflict.
type VersionConflictError struct {
	LatestVersionID string
}

func (e *VersionConflictError) Error() string        { return "version_conflict" }
func (e *VersionConflictError) Is(target error) bool { return target == ErrVersionConflict }

// Comment / transfer status values.
const (
	StatusOpen     = "open"
	StatusPending  = "pending"
	StatusAccepted = "accepted"
	StatusDeclined = "declined"
)

type Document struct {
	ID               string         `json:"id"`
	Title            string         `json:"title"`
	Owner            auth.Principal `json:"owner"`
	CurrentVersionID string         `json:"current_version_id"`
}

type ServiceAccount struct {
	ID       string         `json:"id"`
	Name     string         `json:"name"`
	Owner    auth.Principal `json:"owner"`
	Disabled bool           `json:"disabled"`
}

type ServiceAccountKey struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type OwnershipTransfer struct {
	ID               string `json:"id"`
	ServiceAccountID string `json:"service_account_id"`
	FromUserID       string `json:"from_user_id"`
	ToUserID         string `json:"to_user_id"`
	Status           string `json:"status"`
}

type Grant struct {
	ID         string         `json:"id"`
	DocumentID string         `json:"document_id"`
	Principal  auth.Principal `json:"principal"`
	Role       Role           `json:"role"`
	GrantedBy  auth.Principal `json:"granted_by"`
}

type Version struct {
	ID            string         `json:"id"`
	Number        int            `json:"number"`
	DocumentID    string         `json:"document_id"`
	CreatedBy     auth.Principal `json:"created_by"`
	ChangeSummary string         `json:"change_summary"`
	SHA256        string         `json:"sha256"`
}

type Anchor struct {
	Quote       string `json:"quote"`
	Prefix      string `json:"prefix"`
	Suffix      string `json:"suffix"`
	DOMPath     string `json:"dom_path"`
	StartOffset int    `json:"start_offset"`
	EndOffset   int    `json:"end_offset"`
}

type CLICredential struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type Comment struct {
	ID           string         `json:"id"`
	DocumentID   string         `json:"document_id"`
	VersionID    string         `json:"version_id"`
	Author       auth.Principal `json:"author"`
	Body         string         `json:"body"`
	SelectedText string         `json:"selected_text"`
	Anchor       Anchor         `json:"anchor"`
	Status       string         `json:"status"`
}

// Repository is the application persistence boundary. The real implementation
// will wrap sqlc-generated queries. API tests can provide fakes.
type Repository interface {
	ResolveBearerToken(ctx context.Context, tokenHash string) (auth.Principal, error)
	ResolveUser(ctx context.Context, id string) (auth.Principal, error)
	UpsertGoogleUser(ctx context.Context, id, email, name, googleSub, pictureURL string) (auth.Principal, error)
	CreateCLICredential(ctx context.Context, userID, name, tokenHash string) (string, error)
	ListCLICredentials(ctx context.Context, userID string) ([]CLICredential, error)
	RevokeCLICredential(ctx context.Context, userID, credentialID string) error
	RoleForDocument(ctx context.Context, principal auth.Principal, documentID string) (Role, error)
	CreateDocument(ctx context.Context, owner auth.Principal, title string, html []byte) (Document, Version, error)
	ListDocuments(ctx context.Context, principal auth.Principal) ([]Document, error)
	// RecordDocumentOpened notes that a principal opened a document, so a
	// broadly-shared ("anyone") document joins their workspace listing after
	// first open. Idempotent.
	RecordDocumentOpened(ctx context.Context, documentID string, principal auth.Principal) error
	GetDocument(ctx context.Context, id string) (Document, error)
	UpdateDocument(ctx context.Context, id, title string) (Document, error)
	DeleteDocument(ctx context.Context, id string) error
	CreateServiceAccount(ctx context.Context, owner auth.Principal, name string) (ServiceAccount, error)
	ListServiceAccounts(ctx context.Context, owner auth.Principal) ([]ServiceAccount, error)
	GetServiceAccount(ctx context.Context, id string) (ServiceAccount, error)
	GetServiceAccountByName(ctx context.Context, name string) (ServiceAccount, error)
	UpdateServiceAccount(ctx context.Context, id, name string, disabled *bool) (ServiceAccount, error)
	CreateServiceAccountKey(ctx context.Context, saID, name, tokenHash string) (string, error)
	ListServiceAccountKeys(ctx context.Context, saID string) ([]ServiceAccountKey, error)
	RevokeServiceAccountKey(ctx context.Context, saID, keyID string) error
	CreateOwnershipTransfer(ctx context.Context, saID string, from auth.Principal, toEmail string) (OwnershipTransfer, error)
	ListOwnershipTransfers(ctx context.Context, user auth.Principal) ([]OwnershipTransfer, error)
	AcceptOwnershipTransfer(ctx context.Context, id string, user auth.Principal) (OwnershipTransfer, error)
	DeclineOwnershipTransfer(ctx context.Context, id string, user auth.Principal) error
	PrincipalExists(ctx context.Context, principal auth.Principal) (bool, error)
	EnsureUserByEmail(ctx context.Context, email string) (auth.Principal, error)
	CreateGrant(ctx context.Context, documentID string, principal auth.Principal, role Role, grantedBy auth.Principal) (Grant, error)
	ListGrants(ctx context.Context, documentID string) ([]Grant, error)
	UpdateGrant(ctx context.Context, documentID, grantID string, role Role) (Grant, error)
	DeleteGrant(ctx context.Context, documentID, grantID string) error
	CreateVersion(ctx context.Context, documentID, baseVersionID, changeSummary string, html []byte, createdBy auth.Principal) (Version, error)
	ListVersions(ctx context.Context, documentID string) ([]Version, error)
	GetVersion(ctx context.Context, id string) (Version, error)
	GetVersionHTML(ctx context.Context, id string) ([]byte, error)
	CreateComment(ctx context.Context, documentID, versionID, body string, anchor Anchor, author auth.Principal) (Comment, error)
	GetComment(ctx context.Context, id string) (Comment, error)
	ListComments(ctx context.Context, documentID, status, versionID string) ([]Comment, error)
	UpdateComment(ctx context.Context, id, body, status string) (Comment, error)
	DeleteComment(ctx context.Context, id string) error
}

type Memory struct {
	mu        sync.Mutex
	docN      int
	verN      int
	saN       int
	grantN    int
	commentN  int
	credN     int
	sakN      int
	docs      map[string]Document
	versions  map[string]Version
	grants    []Grant
	users     map[string]auth.Principal
	creds     map[string]auth.Principal
	cliCreds  []cliCredRecord
	sas       map[string]ServiceAccount
	comments  map[string]Comment
	transfers map[string]OwnershipTransfer

	// Test-only fixtures, populated by NewMemorySeeded and consulted by
	// RoleForDocument / PrincipalExists. Kept out of the users/grants stores so
	// they do not perturb id sequencing in handler tests. Always nil/empty in
	// production (NewMemory), so the lookups below are inert there.
	fixtureRoles      map[string]Role
	fixturePrincipals map[string]bool
}

type cliCredRecord struct {
	ID        string
	UserID    string
	TokenHash string
	Name      string
}

// NewMemory returns an empty in-memory repository. It carries no fixtures so it
// behaves like a freshly migrated database; tests that need the canonical
// fixture graph call NewMemorySeeded instead.
func NewMemory() *Memory {
	return &Memory{docs: map[string]Document{}, versions: map[string]Version{}, users: map[string]auth.Principal{}, creds: map[string]auth.Principal{}, sas: map[string]ServiceAccount{}, comments: map[string]Comment{}, transfers: map[string]OwnershipTransfer{}}
}

func (m *Memory) ResolveBearerToken(ctx context.Context, tokenHash string) (auth.Principal, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	p, ok := m.creds[tokenHash]
	if !ok {
		return auth.Principal{}, ErrNotFound
	}
	return p, nil
}

func (m *Memory) ResolveUser(ctx context.Context, id string) (auth.Principal, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	p, ok := m.users[id]
	if !ok {
		return auth.Principal{}, ErrNotFound
	}
	return p, nil
}

func (m *Memory) UpsertGoogleUser(ctx context.Context, id, email, name, googleSub, pictureURL string) (auth.Principal, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for existingID, existing := range m.users {
		if strings.EqualFold(existing.Email, email) {
			existing.Name = name
			existing.PictureURL = pictureURL
			m.users[existingID] = existing
			return existing, nil
		}
	}
	p := auth.Principal{Type: auth.PrincipalUser, ID: id, Email: email, Name: name, PictureURL: pictureURL}
	m.users[id] = p
	return p, nil
}

func (m *Memory) CreateCLICredential(ctx context.Context, userID, name, tokenHash string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	p, ok := m.users[userID]
	if !ok {
		p = auth.Principal{Type: auth.PrincipalUser, ID: userID}
	}
	m.credN++
	id := fmt.Sprintf("cred_%d", m.credN)
	m.creds[tokenHash] = p
	m.cliCreds = append(m.cliCreds, cliCredRecord{ID: id, UserID: userID, TokenHash: tokenHash, Name: name})
	return id, nil
}

func (m *Memory) ListCLICredentials(ctx context.Context, userID string) ([]CLICredential, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := []CLICredential{}
	for _, c := range m.cliCreds {
		if c.UserID == userID {
			out = append(out, CLICredential{ID: c.ID, Name: c.Name})
		}
	}
	return out, nil
}
func (m *Memory) RevokeCLICredential(ctx context.Context, userID, credentialID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for i, c := range m.cliCreds {
		if c.ID == credentialID && c.UserID == userID {
			delete(m.creds, c.TokenHash)
			m.cliCreds = append(m.cliCreds[:i], m.cliCreds[i+1:]...)
			return nil
		}
	}
	return ErrNotFound
}

func (m *Memory) RoleForDocument(ctx context.Context, p auth.Principal, documentID string) (Role, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.docs[documentID]; !ok {
		// Fall through to fixture handling below for API tests that seed roles
		// without a backing document.
		if role, ok := m.fixtureRoles[p.ID]; ok {
			return role, nil
		}
		return "", ErrNotFound
	}
	best := RoleNone
	if d := m.docs[documentID]; p.Type == auth.PrincipalUser && p.ID == d.Owner.ID {
		best = RoleOwner
	}
	for _, g := range m.grants {
		if g.DocumentID != documentID {
			continue
		}
		// An explicit grant matches this exact principal; an "anyone" grant
		// matches every audience.
		if (g.Principal.Type == p.Type && g.Principal.ID == p.ID) || g.Principal.Type == auth.PrincipalAnyone {
			best = MaxRole(best, g.Role)
		}
	}
	if role, ok := m.fixtureRoles[p.ID]; ok {
		best = MaxRole(best, role)
	}
	if best == RoleNone {
		return "", ErrNotFound
	}
	return best, nil
}

func (m *Memory) CreateDocument(ctx context.Context, owner auth.Principal, title string, html []byte) (Document, Version, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.docN++
	m.verN++
	d := Document{ID: fmt.Sprintf("doc_%d", m.docN), Title: title, Owner: owner, CurrentVersionID: fmt.Sprintf("ver_%d", m.verN)}
	v := Version{ID: d.CurrentVersionID, Number: 1, DocumentID: d.ID, CreatedBy: owner, SHA256: fmt.Sprintf("sha_%d", m.verN)}
	m.docs[d.ID] = d
	m.versions[v.ID] = v
	return d, v, nil
}

func (m *Memory) GetDocument(ctx context.Context, id string) (Document, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	d, ok := m.docs[id]
	if !ok {
		return Document{}, ErrNotFound
	}
	return d, nil
}
func (m *Memory) CreateServiceAccount(ctx context.Context, owner auth.Principal, name string) (ServiceAccount, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, sa := range m.sas {
		if sa.Name == name {
			return ServiceAccount{}, ErrConflict
		}
	}
	m.saN++
	sa := ServiceAccount{ID: fmt.Sprintf("sa_%d", m.saN), Name: name, Owner: owner}
	m.sas[sa.ID] = sa
	return sa, nil
}
func (m *Memory) GetServiceAccount(ctx context.Context, id string) (ServiceAccount, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	sa, ok := m.sas[id]
	if !ok {
		return ServiceAccount{}, ErrNotFound
	}
	return sa, nil
}
func (m *Memory) GetServiceAccountByName(ctx context.Context, name string) (ServiceAccount, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, sa := range m.sas {
		if sa.Name == name {
			return sa, nil
		}
	}
	return ServiceAccount{}, ErrNotFound
}
func (m *Memory) PrincipalExists(ctx context.Context, p auth.Principal) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	switch p.Type {
	case auth.PrincipalUser:
		if _, ok := m.users[p.ID]; ok {
			return true, nil
		}
		if m.fixturePrincipals[p.ID] {
			return true, nil
		}
	case auth.PrincipalServiceAccount:
		_, ok := m.sas[p.ID]
		return ok, nil
	}
	return false, nil
}
func (m *Memory) EnsureUserByEmail(ctx context.Context, email string) (auth.Principal, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, p := range m.users {
		if strings.EqualFold(p.Email, email) {
			return p, nil
		}
	}
	id := fmt.Sprintf("usr_%d", len(m.users)+1)
	p := auth.Principal{Type: auth.PrincipalUser, ID: id, Email: email}
	m.users[id] = p
	return p, nil
}
func (m *Memory) CreateGrant(ctx context.Context, documentID string, principal auth.Principal, role Role, grantedBy auth.Principal) (Grant, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.grantN++
	g := Grant{ID: fmt.Sprintf("gr_%d", m.grantN), DocumentID: documentID, Principal: principal, Role: role, GrantedBy: grantedBy}
	m.grants = append(m.grants, g)
	return g, nil
}
func (m *Memory) CreateVersion(ctx context.Context, documentID, baseVersionID, changeSummary string, html []byte, createdBy auth.Principal) (Version, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	d, ok := m.docs[documentID]
	if !ok {
		return Version{}, ErrNotFound
	}
	if baseVersionID != d.CurrentVersionID {
		return Version{}, &VersionConflictError{LatestVersionID: d.CurrentVersionID}
	}
	m.verN++
	v := Version{ID: fmt.Sprintf("ver_%d", m.verN), Number: m.verN, DocumentID: documentID, CreatedBy: createdBy, ChangeSummary: changeSummary, SHA256: fmt.Sprintf("sha_%d", m.verN)}
	d.CurrentVersionID = v.ID
	m.docs[documentID] = d
	m.versions[v.ID] = v
	return v, nil
}
func (m *Memory) CreateComment(ctx context.Context, documentID, versionID, body string, anchor Anchor, author auth.Principal) (Comment, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	v, ok := m.versions[versionID]
	if !ok || v.DocumentID != documentID {
		return Comment{}, ErrNotFound
	}
	m.commentN++
	cm := Comment{ID: fmt.Sprintf("cmt_%d", m.commentN), DocumentID: documentID, VersionID: versionID, Author: author, Body: body, SelectedText: anchor.Quote, Anchor: anchor, Status: StatusOpen}
	m.comments[cm.ID] = cm
	return cm, nil
}
func (m *Memory) GetComment(ctx context.Context, id string) (Comment, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	cm, ok := m.comments[id]
	if !ok {
		return Comment{}, ErrNotFound
	}
	return cm, nil
}

func (m *Memory) ListDocuments(ctx context.Context, principal auth.Principal) ([]Document, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := []Document{}
	for _, d := range m.docs {
		out = append(out, d)
	}
	return out, nil
}
func (m *Memory) RecordDocumentOpened(ctx context.Context, documentID string, principal auth.Principal) error {
	return nil
}
func (m *Memory) UpdateDocument(ctx context.Context, id, title string) (Document, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	d, ok := m.docs[id]
	if !ok {
		return Document{}, ErrNotFound
	}
	if title != "" {
		d.Title = title
	}
	m.docs[id] = d
	return d, nil
}
func (m *Memory) DeleteDocument(ctx context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.docs, id)
	return nil
}
func (m *Memory) ListServiceAccounts(ctx context.Context, owner auth.Principal) ([]ServiceAccount, error) {
	return []ServiceAccount{}, nil
}
func (m *Memory) UpdateServiceAccount(ctx context.Context, id, name string, disabled *bool) (ServiceAccount, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	sa, ok := m.sas[id]
	if !ok {
		return ServiceAccount{}, ErrNotFound
	}
	if name != "" {
		sa.Name = name
	}
	if disabled != nil {
		sa.Disabled = *disabled
	}
	m.sas[id] = sa
	return sa, nil
}
func (m *Memory) CreateServiceAccountKey(ctx context.Context, saID, name, tokenHash string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sakN++
	id := fmt.Sprintf("sak_%d", m.sakN)
	// The principal represents the service account identity; its Name is the
	// service account's name (not the SA id, and not the key name).
	saName := ""
	if sa, ok := m.sas[saID]; ok {
		saName = sa.Name
	}
	m.creds[tokenHash] = auth.Principal{Type: auth.PrincipalServiceAccount, ID: saID, Name: saName}
	return id, nil
}
func (m *Memory) ListServiceAccountKeys(ctx context.Context, saID string) ([]ServiceAccountKey, error) {
	return []ServiceAccountKey{}, nil
}
func (m *Memory) RevokeServiceAccountKey(ctx context.Context, saID, keyID string) error { return nil }
func (m *Memory) ListGrants(ctx context.Context, documentID string) ([]Grant, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := []Grant{}
	for _, g := range m.grants {
		if g.DocumentID == documentID {
			out = append(out, g)
		}
	}
	return out, nil
}
func (m *Memory) UpdateGrant(ctx context.Context, documentID, grantID string, role Role) (Grant, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for i, g := range m.grants {
		if g.ID == grantID && g.DocumentID == documentID {
			m.grants[i].Role = role
			return m.grants[i], nil
		}
	}
	return Grant{}, ErrNotFound
}
func (m *Memory) DeleteGrant(ctx context.Context, documentID, grantID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for i, g := range m.grants {
		if g.ID == grantID && g.DocumentID == documentID {
			m.grants = append(m.grants[:i], m.grants[i+1:]...)
			return nil
		}
	}
	return ErrNotFound
}
func (m *Memory) ListVersions(ctx context.Context, documentID string) ([]Version, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := []Version{}
	for _, v := range m.versions {
		if v.DocumentID == documentID {
			out = append(out, v)
		}
	}
	return out, nil
}
func (m *Memory) GetVersion(ctx context.Context, id string) (Version, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	v, ok := m.versions[id]
	if !ok {
		return Version{}, ErrNotFound
	}
	return v, nil
}
func (m *Memory) GetVersionHTML(ctx context.Context, id string) ([]byte, error) {
	return []byte("<html></html>"), nil
}
func (m *Memory) ListComments(ctx context.Context, documentID, status, versionID string) ([]Comment, error) {
	return []Comment{}, nil
}
func (m *Memory) UpdateComment(ctx context.Context, id, body, status string) (Comment, error) {
	return Comment{ID: id, Body: body, Status: status}, nil
}
func (m *Memory) DeleteComment(ctx context.Context, id string) error { return nil }

func (m *Memory) CreateOwnershipTransfer(ctx context.Context, saID string, from auth.Principal, toEmail string) (OwnershipTransfer, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	// Resolve the target email to a stable user ID so ToUserID has the same
	// meaning as the Postgres implementation (which stores a resolved user id,
	// not an email). AcceptOwnershipTransfer compares ToUserID to user.ID.
	toID := m.resolveUserIDByEmailLocked(toEmail)
	id := fmt.Sprintf("xfer_%d", len(m.transfers)+1)
	x := OwnershipTransfer{ID: id, ServiceAccountID: saID, FromUserID: from.ID, ToUserID: toID, Status: StatusPending}
	m.transfers[id] = x
	return x, nil
}

// resolveUserIDByEmailLocked returns the id of an existing user with the given
// email, creating a placeholder user if none exists. Caller must hold m.mu.
func (m *Memory) resolveUserIDByEmailLocked(email string) string {
	for _, p := range m.users {
		if strings.EqualFold(p.Email, email) {
			return p.ID
		}
	}
	id := fmt.Sprintf("usr_%d", len(m.users)+1)
	m.users[id] = auth.Principal{Type: auth.PrincipalUser, ID: id, Email: email}
	return id
}
func (m *Memory) ListOwnershipTransfers(ctx context.Context, user auth.Principal) ([]OwnershipTransfer, error) {
	return []OwnershipTransfer{}, nil
}
func (m *Memory) AcceptOwnershipTransfer(ctx context.Context, id string, user auth.Principal) (OwnershipTransfer, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	x, ok := m.transfers[id]
	if !ok {
		return OwnershipTransfer{}, ErrNotFound
	}
	if x.ToUserID != user.ID {
		return x, ErrNotTransferTarget
	}
	if x.Status != StatusPending {
		return x, ErrTransferNotPending
	}
	x.Status = StatusAccepted
	m.transfers[id] = x
	return x, nil
}
func (m *Memory) DeclineOwnershipTransfer(ctx context.Context, id string, user auth.Principal) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	x, ok := m.transfers[id]
	if !ok || (x.FromUserID != user.ID && x.ToUserID != user.ID) {
		return ErrNotFound
	}
	x.Status = StatusDeclined
	m.transfers[id] = x
	return nil
}
