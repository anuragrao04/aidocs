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
	RoleViewer    Role = "viewer"
	RoleCommenter Role = "commenter"
	RoleEditor    Role = "editor"
	RoleOwner     Role = "owner"
)

var ErrNotFound = errors.New("not found")

type Document struct {
	ID               string
	Title            string
	Visibility       string
	Owner            auth.Principal
	CurrentVersionID string
}

type ServiceAccount struct {
	ID       string
	Name     string
	Owner    auth.Principal
	Disabled bool
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
	ID         string
	DocumentID string
	Principal  auth.Principal
	Role       Role
	GrantedBy  auth.Principal
}

type Version struct {
	ID            string
	Number        int
	DocumentID    string
	CreatedBy     auth.Principal
	ChangeSummary string
	SHA256        string
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
	ID   string
	Name string
}

type Comment struct {
	ID           string
	DocumentID   string
	VersionID    string
	Author       auth.Principal
	Body         string
	SelectedText string
	Anchor       Anchor
	Status       string
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
	CreateDocument(ctx context.Context, owner auth.Principal, title, visibility string, html []byte) (Document, Version, error)
	ListDocuments(ctx context.Context, principal auth.Principal) ([]Document, error)
	GetDocument(ctx context.Context, id string) (Document, error)
	UpdateDocument(ctx context.Context, id, title, visibility string) (Document, error)
	DeleteDocument(ctx context.Context, id string) error
	CreateServiceAccount(ctx context.Context, owner auth.Principal, name string) (ServiceAccount, error)
	ListServiceAccounts(ctx context.Context, owner auth.Principal) ([]ServiceAccount, error)
	GetServiceAccount(ctx context.Context, id string) (ServiceAccount, error)
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
	docs      map[string]Document
	versions  map[string]Version
	grants    []Grant
	users     map[string]auth.Principal
	creds     map[string]auth.Principal
	sas       map[string]ServiceAccount
	comments  map[string]Comment
	transfers map[string]OwnershipTransfer
}

func NewMemory() *Memory {
	m := &Memory{docs: map[string]Document{}, versions: map[string]Version{}, users: map[string]auth.Principal{}, creds: map[string]auth.Principal{}, sas: map[string]ServiceAccount{}, comments: map[string]Comment{}, transfers: map[string]OwnershipTransfer{}}
	owner := auth.Principal{Type: auth.PrincipalUser, ID: "owner_1", Email: "owner@example.com", Name: "Owner"}
	m.users[owner.ID] = owner
	m.docs["doc_1"] = Document{ID: "doc_1", Title: "fixture", Visibility: "private", Owner: owner, CurrentVersionID: "ver_1"}
	m.versions["ver_1"] = Version{ID: "ver_1", Number: 1, DocumentID: "doc_1", CreatedBy: owner, SHA256: "sha_1"}
	m.sas["sa_1"] = ServiceAccount{ID: "sa_1", Name: "fixture", Owner: owner}
	m.comments["cmt_1"] = Comment{ID: "cmt_1", DocumentID: "doc_1", VersionID: "ver_1", Author: auth.Principal{Type: auth.PrincipalUser, ID: "commenter_1", Email: "commenter@example.com", Name: "Commenter"}, Body: "original", Status: "open"}
	return m
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
	id := fmt.Sprintf("cred_%d", len(m.creds)+1)
	m.creds[tokenHash] = p
	return id, nil
}

func (m *Memory) ListCLICredentials(ctx context.Context, userID string) ([]CLICredential, error) {
	return []CLICredential{}, nil
}
func (m *Memory) RevokeCLICredential(ctx context.Context, userID, credentialID string) error {
	return nil
}

func (m *Memory) RoleForDocument(ctx context.Context, p auth.Principal, documentID string) (Role, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	d, ok := m.docs[documentID]
	if ok && p.Type == auth.PrincipalUser && p.ID == d.Owner.ID {
		return RoleOwner, nil
	}
	for _, g := range m.grants {
		if g.DocumentID == documentID && g.Principal.Type == p.Type && g.Principal.ID == p.ID {
			return g.Role, nil
		}
	}
	// Development fixture convention for API tests.
	switch p.ID {
	case "owner_1":
		return RoleOwner, nil
	case "editor_1":
		return RoleEditor, nil
	case "commenter_1":
		return RoleCommenter, nil
	case "viewer_1":
		return RoleViewer, nil
	}
	return "", ErrNotFound
}

func (m *Memory) CreateDocument(ctx context.Context, owner auth.Principal, title, visibility string, html []byte) (Document, Version, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.docN++
	m.verN++
	d := Document{ID: fmt.Sprintf("doc_%d", m.docN), Title: title, Visibility: visibility, Owner: owner, CurrentVersionID: fmt.Sprintf("ver_%d", m.verN)}
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
func (m *Memory) PrincipalExists(ctx context.Context, p auth.Principal) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	switch p.Type {
	case auth.PrincipalUser:
		if _, ok := m.users[p.ID]; ok || p.ID == "owner_1" || p.ID == "viewer_1" || p.ID == "editor_1" || p.ID == "commenter_1" {
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
	if !ok && documentID == "doc_1" {
		d = Document{ID: "doc_1", CurrentVersionID: "ver_1"}
		ok = true
	}
	if !ok {
		return Version{}, ErrNotFound
	}
	if baseVersionID != d.CurrentVersionID {
		return Version{ID: d.CurrentVersionID}, errors.New("version_conflict")
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
	cm := Comment{ID: fmt.Sprintf("cmt_%d", m.commentN), DocumentID: documentID, VersionID: versionID, Author: author, Body: body, SelectedText: anchor.Quote, Anchor: anchor, Status: "open"}
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
func (m *Memory) UpdateDocument(ctx context.Context, id, title, visibility string) (Document, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	d, ok := m.docs[id]
	if !ok {
		return Document{}, ErrNotFound
	}
	if title != "" {
		d.Title = title
	}
	if visibility != "" {
		d.Visibility = visibility
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
	return ServiceAccount{ID: id, Name: name, Disabled: disabled != nil && *disabled}, nil
}
func (m *Memory) CreateServiceAccountKey(ctx context.Context, saID, name, tokenHash string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	id := fmt.Sprintf("sak_%d", len(m.creds)+1)
	m.creds[tokenHash] = auth.Principal{Type: auth.PrincipalServiceAccount, ID: saID, Name: saID}
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
	id := fmt.Sprintf("xfer_%d", len(m.transfers)+1)
	x := OwnershipTransfer{ID: id, ServiceAccountID: saID, FromUserID: from.ID, ToUserID: toEmail, Status: "pending"}
	m.transfers[id] = x
	return x, nil
}
func (m *Memory) ListOwnershipTransfers(ctx context.Context, user auth.Principal) ([]OwnershipTransfer, error) {
	return []OwnershipTransfer{}, nil
}
func (m *Memory) AcceptOwnershipTransfer(ctx context.Context, id string, user auth.Principal) (OwnershipTransfer, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	x, ok := m.transfers[id]
	if !ok || x.ToUserID != user.ID || x.Status != "pending" {
		return x, errors.New("not allowed")
	}
	x.Status = "accepted"
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
	x.Status = "declined"
	m.transfers[id] = x
	return nil
}
