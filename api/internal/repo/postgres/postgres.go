package postgres

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/anuragrao/aidocs/api/internal/auth"
	"github.com/anuragrao/aidocs/api/internal/blob"
	dbsqlc "github.com/anuragrao/aidocs/api/internal/db/sqlc"
	"github.com/anuragrao/aidocs/api/internal/repo"
)

// htmlContentType is the content type stored for all version HTML blobs.
const htmlContentType = "text/html; charset=utf-8"

type Store struct {
	db    *pgxpool.Pool
	q     *dbsqlc.Queries
	blobs blob.Store
}

func New(db *pgxpool.Pool) *Store { return NewWithBlob(db, nil) }
func NewWithBlob(db *pgxpool.Pool, blobs blob.Store) *Store {
	if blobs == nil {
		// Tests/local callers that do not wire S3 still use a blob store;
		// version HTML is never duplicated in Postgres.
		blobs = blob.NewMemory()
	}
	return &Store{db: db, q: dbsqlc.New(db), blobs: blobs}
}
func Connect(ctx context.Context, databaseURL string) (*Store, error) {
	return ConnectWithBlob(ctx, databaseURL, nil)
}
func ConnectWithBlob(ctx context.Context, databaseURL string, blobs blob.Store) (*Store, error) {
	p, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return nil, err
	}
	if err := p.Ping(ctx); err != nil {
		p.Close()
		return nil, err
	}
	return NewWithBlob(p, blobs), nil
}
func (s *Store) Close()              { s.db.Close() }
func (s *Store) Pool() *pgxpool.Pool { return s.db }

func (s *Store) ResolveBearerToken(ctx context.Context, tokenHash string) (auth.Principal, error) {
	u, err := s.q.ResolveCLIToken(ctx, tokenHash)
	if err == nil {
		_ = s.q.TouchCLIToken(ctx, tokenHash)
		return auth.Principal{Type: auth.PrincipalType(u.PrincipalType), ID: u.ID, Email: u.Email, Name: u.Name, PictureURL: u.PictureUrl}, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return auth.Principal{}, err
	}
	sa, err := s.q.ResolveServiceAccountToken(ctx, tokenHash)
	if errors.Is(err, pgx.ErrNoRows) {
		return auth.Principal{}, repo.ErrNotFound
	}
	if err == nil {
		_ = s.q.TouchServiceAccountToken(ctx, tokenHash)
		return auth.Principal{Type: auth.PrincipalType(sa.PrincipalType), ID: sa.ID, Email: sa.Email, Name: sa.Name, PictureURL: sa.PictureUrl}, nil
	}
	return auth.Principal{}, err
}
func (s *Store) ResolveUser(ctx context.Context, id string) (auth.Principal, error) {
	u, err := s.q.GetUserByID(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return auth.Principal{}, repo.ErrNotFound
	}
	if err != nil {
		return auth.Principal{}, err
	}
	return auth.Principal{Type: auth.PrincipalUser, ID: u.ID, Email: u.Email, Name: u.Name, PictureURL: u.PictureUrl}, nil
}
func (s *Store) UpsertGoogleUser(ctx context.Context, id, email, name, googleSub, pictureURL string) (auth.Principal, error) {
	err := s.q.UpsertGoogleUser(ctx, dbsqlc.UpsertGoogleUserParams{ID: id, Email: email, Name: name, GoogleSub: pgtype.Text{String: googleSub, Valid: true}, PictureUrl: pictureURL})
	if err != nil {
		return auth.Principal{}, err
	}
	u, err := s.q.GetUserByGoogleSub(ctx, pgtype.Text{String: googleSub, Valid: true})
	if err != nil {
		return auth.Principal{}, err
	}
	return auth.Principal{Type: auth.PrincipalUser, ID: u.ID, Email: u.Email, Name: u.Name, PictureURL: u.PictureUrl}, nil
}
func (s *Store) EnsureUserByEmail(ctx context.Context, email string) (auth.Principal, error) {
	u, err := s.q.GetUserByEmail(ctx, email)
	if err == nil {
		return auth.Principal{Type: auth.PrincipalUser, ID: u.ID, Email: u.Email, Name: u.Name, PictureURL: u.PictureUrl}, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return auth.Principal{}, err
	}
	id := newID("usr")
	if err := s.q.InsertPlaceholderUser(ctx, dbsqlc.InsertPlaceholderUserParams{ID: id, Email: email}); err != nil {
		return auth.Principal{}, err
	}
	return auth.Principal{Type: auth.PrincipalUser, ID: id, Email: email}, nil
}
func (s *Store) CreateCLICredential(ctx context.Context, userID, name, tokenHash string) (string, error) {
	id := newID("cred")
	err := s.q.CreateCLICredential(ctx, dbsqlc.CreateCLICredentialParams{ID: id, UserID: userID, Name: name, TokenHash: tokenHash})
	return id, err
}
func (s *Store) ListCLICredentials(ctx context.Context, userID string) ([]repo.CLICredential, error) {
	rows, err := s.q.ListCLICredentials(ctx, userID)
	if err != nil {
		return nil, err
	}
	out := make([]repo.CLICredential, 0, len(rows))
	for _, r := range rows {
		out = append(out, repo.CLICredential{ID: r.ID, Name: r.Name})
	}
	return out, nil
}
func (s *Store) RevokeCLICredential(ctx context.Context, userID, credentialID string) error {
	return s.q.RevokeCLICredential(ctx, dbsqlc.RevokeCLICredentialParams{ID: credentialID, UserID: userID})
}

func (s *Store) RoleForDocument(ctx context.Context, p auth.Principal, documentID string) (repo.Role, error) {
	ownerID, err := s.q.GetDocumentOwnerID(ctx, documentID)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", repo.ErrNotFound
	}
	if err != nil {
		return "", err
	}
	best := repo.RoleNone
	if p.Type == auth.PrincipalUser && p.ID == ownerID {
		best = repo.RoleOwner
	}
	// Explicit grant for this exact principal.
	if role, err := s.grantRole(ctx, documentID, string(p.Type), p.ID); err != nil {
		return "", err
	} else {
		best = repo.MaxRole(best, role)
	}
	// "anyone" grant applies to every audience that reached this server.
	if role, err := s.grantRole(ctx, documentID, string(auth.PrincipalAnyone), ""); err != nil {
		return "", err
	} else {
		best = repo.MaxRole(best, role)
	}
	if best == repo.RoleNone {
		return "", repo.ErrNotFound
	}
	return best, nil
}

// grantRole returns the role of a single grant, or RoleNone if none exists.
func (s *Store) grantRole(ctx context.Context, documentID, principalType, principalID string) (repo.Role, error) {
	role, err := s.q.GetDocumentGrantRole(ctx, dbsqlc.GetDocumentGrantRoleParams{ResourceID: documentID, PrincipalType: principalType, PrincipalID: principalID})
	if errors.Is(err, pgx.ErrNoRows) {
		return repo.RoleNone, nil
	}
	if err != nil {
		return repo.RoleNone, err
	}
	return repo.Role(role), nil
}

func (s *Store) CreateDocument(ctx context.Context, owner auth.Principal, title string, html []byte) (repo.Document, repo.Version, error) {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return repo.Document{}, repo.Version{}, err
	}
	defer tx.Rollback(ctx)
	q := s.q.WithTx(tx)
	if err := upsertUser(ctx, q, owner); err != nil {
		return repo.Document{}, repo.Version{}, err
	}
	docID, verID := newID("doc"), newID("ver")
	blobKey := fmt.Sprintf("docs/%s/%s.html", docID, verID)
	sum := sha(html)
	if err := s.blobs.Put(ctx, blobKey, htmlContentType, html); err != nil {
		return repo.Document{}, repo.Version{}, err
	}
	committed := false
	defer func() {
		if !committed {
			_ = s.blobs.Delete(context.WithoutCancel(ctx), blobKey)
		}
	}()
	if err := q.InsertDocument(ctx, dbsqlc.InsertDocumentParams{ID: docID, Title: title, OwnerID: owner.ID}); err != nil {
		return repo.Document{}, repo.Version{}, err
	}
	if err := q.InsertInitialVersion(ctx, dbsqlc.InsertInitialVersionParams{ID: verID, DocumentID: docID, HtmlBlobKey: blobKey, Sha256: sum, CreatedByType: string(owner.Type), CreatedByID: owner.ID}); err != nil {
		return repo.Document{}, repo.Version{}, err
	}
	if err := q.UpdateDocumentCurrentVersion(ctx, dbsqlc.UpdateDocumentCurrentVersionParams{CurrentVersionID: pgtype.Text{String: verID, Valid: true}, ID: docID}); err != nil {
		return repo.Document{}, repo.Version{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return repo.Document{}, repo.Version{}, err
	}
	committed = true
	return repo.Document{ID: docID, Title: title, Owner: owner, CurrentVersionID: verID}, repo.Version{ID: verID, Number: 1, DocumentID: docID, CreatedBy: owner, SHA256: sum}, nil
}
func (s *Store) GetDocument(ctx context.Context, id string) (repo.Document, error) {
	r, err := s.q.GetDocument(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return repo.Document{}, repo.ErrNotFound
	}
	if err != nil {
		return repo.Document{}, err
	}
	return docFromGet(r), nil
}

func (s *Store) CreateServiceAccount(ctx context.Context, owner auth.Principal, name string) (repo.ServiceAccount, error) {
	if owner.Type != auth.PrincipalUser {
		return repo.ServiceAccount{}, errors.New("owner must be user")
	}
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return repo.ServiceAccount{}, err
	}
	defer tx.Rollback(ctx)
	q := s.q.WithTx(tx)
	if err := upsertUser(ctx, q, owner); err != nil {
		return repo.ServiceAccount{}, err
	}
	id := newID("sa")
	if err := q.InsertServiceAccount(ctx, dbsqlc.InsertServiceAccountParams{ID: id, Name: name, OwnerUserID: owner.ID}); err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return repo.ServiceAccount{}, repo.ErrConflict
		}
		return repo.ServiceAccount{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return repo.ServiceAccount{}, err
	}
	return repo.ServiceAccount{ID: id, Name: name, Owner: owner}, nil
}
func (s *Store) GetServiceAccount(ctx context.Context, id string) (repo.ServiceAccount, error) {
	r, err := s.q.GetServiceAccount(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return repo.ServiceAccount{}, repo.ErrNotFound
	}
	if err != nil {
		return repo.ServiceAccount{}, err
	}
	return repo.ServiceAccount{ID: r.ID, Name: r.Name, Disabled: r.Disabled, Owner: auth.Principal{Type: auth.PrincipalUser, ID: r.OwnerID, Email: r.OwnerEmail, Name: r.OwnerName}}, nil
}
func (s *Store) GetServiceAccountByName(ctx context.Context, name string) (repo.ServiceAccount, error) {
	r, err := s.q.GetServiceAccountByName(ctx, name)
	if errors.Is(err, pgx.ErrNoRows) {
		return repo.ServiceAccount{}, repo.ErrNotFound
	}
	if err != nil {
		return repo.ServiceAccount{}, err
	}
	return repo.ServiceAccount{ID: r.ID, Name: r.Name, Disabled: r.Disabled, Owner: auth.Principal{Type: auth.PrincipalUser, ID: r.OwnerID, Email: r.OwnerEmail, Name: r.OwnerName}}, nil
}
func (s *Store) PrincipalExists(ctx context.Context, p auth.Principal) (bool, error) {
	switch p.Type {
	case auth.PrincipalUser:
		return s.q.UserExists(ctx, p.ID)
	case auth.PrincipalServiceAccount:
		return s.q.ServiceAccountExists(ctx, p.ID)
	default:
		return false, nil
	}
}
func (s *Store) CreateGrant(ctx context.Context, documentID string, principal auth.Principal, role repo.Role, grantedBy auth.Principal) (repo.Grant, error) {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return repo.Grant{}, err
	}
	defer tx.Rollback(ctx)
	q := s.q.WithTx(tx)
	if grantedBy.Type == auth.PrincipalUser {
		if err := upsertUser(ctx, q, grantedBy); err != nil {
			return repo.Grant{}, err
		}
	}
	if principal.Type == auth.PrincipalUser {
		if err := upsertUser(ctx, q, principal); err != nil {
			return repo.Grant{}, err
		}
	}
	// The upsert returns the authoritative id (the existing row's id on
	// conflict, or the newly generated one on insert), so we trust its result
	// rather than keeping a second, possibly-discarded local id.
	id, err := q.UpsertResourceGrant(ctx, dbsqlc.UpsertResourceGrantParams{ID: newID("gr"), ResourceID: documentID, PrincipalType: string(principal.Type), PrincipalID: principal.ID, Role: string(role), GrantedByUserID: grantedBy.ID})
	if err != nil {
		return repo.Grant{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return repo.Grant{}, err
	}
	return repo.Grant{ID: id, DocumentID: documentID, Principal: principal, Role: role, GrantedBy: grantedBy}, nil
}

func (s *Store) CreateVersion(ctx context.Context, documentID, baseVersionID, changeSummary string, html []byte, createdBy auth.Principal) (repo.Version, error) {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return repo.Version{}, err
	}
	defer tx.Rollback(ctx)
	q := s.q.WithTx(tx)
	cur, err := q.GetDocumentVersionForUpdate(ctx, documentID)
	if errors.Is(err, pgx.ErrNoRows) {
		return repo.Version{}, repo.ErrNotFound
	}
	if err != nil {
		return repo.Version{}, err
	}
	if cur.CurrentVersionID.String != baseVersionID {
		return repo.Version{ID: cur.CurrentVersionID.String}, &repo.VersionConflictError{LatestVersionID: cur.CurrentVersionID.String}
	}
	id := newID("ver")
	blobKey := fmt.Sprintf("docs/%s/%s.html", documentID, id)
	sum := sha(html)
	if err := s.blobs.Put(ctx, blobKey, htmlContentType, html); err != nil {
		return repo.Version{}, err
	}
	committed := false
	defer func() {
		if !committed {
			_ = s.blobs.Delete(context.WithoutCancel(ctx), blobKey)
		}
	}()
	if err := q.InsertVersion(ctx, dbsqlc.InsertVersionParams{ID: id, DocumentID: documentID, Number: cur.NextNumber, HtmlBlobKey: blobKey, Sha256: sum, ParentVersionID: pgtype.Text{String: cur.CurrentVersionID.String, Valid: cur.CurrentVersionID.Valid}, CreatedByType: string(createdBy.Type), CreatedByID: createdBy.ID, ChangeSummary: changeSummary}); err != nil {
		return repo.Version{}, err
	}
	if err := q.UpdateDocumentCurrentVersion(ctx, dbsqlc.UpdateDocumentCurrentVersionParams{CurrentVersionID: pgtype.Text{String: id, Valid: true}, ID: documentID}); err != nil {
		return repo.Version{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return repo.Version{}, err
	}
	committed = true
	return repo.Version{ID: id, Number: int(cur.NextNumber), DocumentID: documentID, CreatedBy: createdBy, ChangeSummary: changeSummary, SHA256: sum}, nil
}
func (s *Store) CreateComment(ctx context.Context, documentID, versionID, body string, anchor repo.Anchor, author auth.Principal) (repo.Comment, error) {
	v, err := s.GetVersion(ctx, versionID)
	if err != nil {
		return repo.Comment{}, err
	}
	if v.DocumentID != documentID {
		return repo.Comment{}, repo.ErrNotFound
	}
	b, err := json.Marshal(anchor)
	if err != nil {
		return repo.Comment{}, err
	}
	id := newID("cmt")
	err = s.q.InsertComment(ctx, dbsqlc.InsertCommentParams{ID: id, DocumentID: documentID, CreatedOnVersionID: versionID, AuthorType: string(author.Type), AuthorID: author.ID, AuthorEmail: author.Email, AuthorName: author.Name, Body: body, SelectedText: anchor.Quote, AnchorJson: b})
	if err != nil {
		return repo.Comment{}, err
	}
	return repo.Comment{ID: id, DocumentID: documentID, VersionID: versionID, Author: author, Body: body, SelectedText: anchor.Quote, Anchor: anchor, Status: repo.StatusOpen}, nil
}
func upsertUser(ctx context.Context, q *dbsqlc.Queries, p auth.Principal) error {
	return q.UpsertUser(ctx, dbsqlc.UpsertUserParams{ID: p.ID, Email: p.Email, Name: p.Name})
}
func sha(b []byte) string { s := sha256.Sum256(b); return hex.EncodeToString(s[:]) }

func (s *Store) ListDocuments(ctx context.Context, p auth.Principal) ([]repo.Document, error) {
	rows, err := s.q.ListDocuments(ctx, dbsqlc.ListDocumentsParams{OwnerID: p.ID, Column2: string(p.Type)})
	if err != nil {
		return nil, err
	}
	out := make([]repo.Document, 0, len(rows))
	for _, r := range rows {
		out = append(out, docFromList(r))
	}
	return out, nil
}
func (s *Store) UpdateDocument(ctx context.Context, id, title string) (repo.Document, error) {
	if title != "" {
		if err := s.q.UpdateDocumentTitle(ctx, dbsqlc.UpdateDocumentTitleParams{Title: title, ID: id}); err != nil {
			return repo.Document{}, err
		}
	}
	return s.GetDocument(ctx, id)
}
func (s *Store) DeleteDocument(ctx context.Context, id string) error {
	return s.q.DeleteDocument(ctx, id)
}
func (s *Store) ListServiceAccounts(ctx context.Context, owner auth.Principal) ([]repo.ServiceAccount, error) {
	rows, err := s.q.ListServiceAccounts(ctx, owner.ID)
	if err != nil {
		return nil, err
	}
	out := make([]repo.ServiceAccount, 0, len(rows))
	for _, r := range rows {
		out = append(out, repo.ServiceAccount{ID: r.ID, Name: r.Name, Disabled: r.Disabled, Owner: auth.Principal{Type: auth.PrincipalUser, ID: r.OwnerID, Email: r.OwnerEmail, Name: r.OwnerName}})
	}
	return out, nil
}
func (s *Store) UpdateServiceAccount(ctx context.Context, id, name string, disabled *bool) (repo.ServiceAccount, error) {
	if name != "" {
		if err := s.q.UpdateServiceAccountName(ctx, dbsqlc.UpdateServiceAccountNameParams{Name: name, ID: id}); err != nil {
			return repo.ServiceAccount{}, err
		}
	}
	if disabled != nil {
		if *disabled {
			if err := s.q.DisableServiceAccount(ctx, id); err != nil {
				return repo.ServiceAccount{}, err
			}
		} else {
			if err := s.q.EnableServiceAccount(ctx, id); err != nil {
				return repo.ServiceAccount{}, err
			}
		}
	}
	return s.GetServiceAccount(ctx, id)
}
func (s *Store) CreateServiceAccountKey(ctx context.Context, saID, name, tokenHash string) (string, error) {
	id := newID("sak")
	err := s.q.InsertServiceAccountKey(ctx, dbsqlc.InsertServiceAccountKeyParams{ID: id, ServiceAccountID: saID, Name: name, TokenHash: tokenHash})
	return id, err
}
func (s *Store) ListServiceAccountKeys(ctx context.Context, saID string) ([]repo.ServiceAccountKey, error) {
	rows, err := s.q.ListServiceAccountKeys(ctx, saID)
	if err != nil {
		return nil, err
	}
	out := make([]repo.ServiceAccountKey, 0, len(rows))
	for _, r := range rows {
		out = append(out, repo.ServiceAccountKey{ID: r.ID, Name: r.Name})
	}
	return out, nil
}
func (s *Store) RevokeServiceAccountKey(ctx context.Context, saID, keyID string) error {
	return s.q.RevokeServiceAccountKey(ctx, dbsqlc.RevokeServiceAccountKeyParams{ServiceAccountID: saID, ID: keyID})
}
func (s *Store) ListGrants(ctx context.Context, documentID string) ([]repo.Grant, error) {
	rows, err := s.q.ListGrants(ctx, documentID)
	if err != nil {
		return nil, err
	}
	out := make([]repo.Grant, 0, len(rows))
	for _, r := range rows {
		out = append(out, repo.Grant{ID: r.ID, DocumentID: documentID, Principal: auth.Principal{Type: auth.PrincipalType(r.PrincipalType), ID: r.PrincipalID}, Role: repo.Role(r.Role), GrantedBy: auth.Principal{Type: auth.PrincipalUser, ID: r.GrantedByUserID}})
	}
	return out, nil
}
func (s *Store) UpdateGrant(ctx context.Context, documentID, grantID string, role repo.Role) (repo.Grant, error) {
	r, err := s.q.UpdateGrantRole(ctx, dbsqlc.UpdateGrantRoleParams{Role: string(role), ID: grantID, ResourceID: documentID})
	if errors.Is(err, pgx.ErrNoRows) {
		return repo.Grant{}, repo.ErrNotFound
	}
	if err != nil {
		return repo.Grant{}, err
	}
	return repo.Grant{ID: r.ID, DocumentID: r.ResourceID, Principal: auth.Principal{Type: auth.PrincipalType(r.PrincipalType), ID: r.PrincipalID}, Role: repo.Role(r.Role), GrantedBy: auth.Principal{Type: auth.PrincipalUser, ID: r.GrantedByUserID}}, nil
}
func (s *Store) DeleteGrant(ctx context.Context, documentID, grantID string) error {
	_, err := s.q.DeleteGrant(ctx, dbsqlc.DeleteGrantParams{ID: grantID, ResourceID: documentID})
	if errors.Is(err, pgx.ErrNoRows) {
		return repo.ErrNotFound
	}
	return err
}
func (s *Store) ListVersions(ctx context.Context, documentID string) ([]repo.Version, error) {
	rows, err := s.q.ListVersions(ctx, documentID)
	if err != nil {
		return nil, err
	}
	out := make([]repo.Version, 0, len(rows))
	for _, r := range rows {
		out = append(out, repo.Version{ID: r.ID, Number: int(r.Number), DocumentID: r.DocumentID, CreatedBy: auth.Principal{Type: auth.PrincipalType(r.CreatedByType), ID: r.CreatedByID}, ChangeSummary: r.ChangeSummary, SHA256: r.Sha256})
	}
	return out, nil
}
func (s *Store) GetVersion(ctx context.Context, id string) (repo.Version, error) {
	r, err := s.q.GetVersion(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return repo.Version{}, repo.ErrNotFound
	}
	if err != nil {
		return repo.Version{}, err
	}
	return repo.Version{ID: r.ID, Number: int(r.Number), DocumentID: r.DocumentID, CreatedBy: auth.Principal{Type: auth.PrincipalType(r.CreatedByType), ID: r.CreatedByID}, ChangeSummary: r.ChangeSummary, SHA256: r.Sha256}, nil
}
func (s *Store) GetVersionHTML(ctx context.Context, id string) ([]byte, error) {
	key, err := s.q.GetVersionBlobKey(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, repo.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	b, _, err := s.blobs.Get(ctx, key)
	if errors.Is(err, blob.ErrNotFound) {
		return nil, repo.ErrNotFound
	}
	return b, err
}
func (s *Store) GetComment(ctx context.Context, id string) (repo.Comment, error) {
	r, err := s.q.GetComment(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return repo.Comment{}, repo.ErrNotFound
	}
	if err != nil {
		return repo.Comment{}, err
	}
	return commentFromGet(r), nil
}
func (s *Store) ListComments(ctx context.Context, documentID, status, versionID string) ([]repo.Comment, error) {
	rows, err := s.q.ListComments(ctx, dbsqlc.ListCommentsParams{DocumentID: documentID, Column2: status, Column3: versionID})
	if err != nil {
		return nil, err
	}
	out := make([]repo.Comment, 0, len(rows))
	for _, r := range rows {
		out = append(out, commentFromList(r))
	}
	return out, nil
}
func (s *Store) UpdateComment(ctx context.Context, id, body, status string) (repo.Comment, error) {
	if body != "" {
		if err := s.q.UpdateCommentBody(ctx, dbsqlc.UpdateCommentBodyParams{Body: body, ID: id}); err != nil {
			return repo.Comment{}, err
		}
	}
	if status != "" {
		if err := s.q.UpdateCommentStatus(ctx, dbsqlc.UpdateCommentStatusParams{Status: status, ID: id}); err != nil {
			return repo.Comment{}, err
		}
	}
	return s.GetComment(ctx, id)
}
func (s *Store) DeleteComment(ctx context.Context, id string) error {
	return s.q.DeleteComment(ctx, id)
}

func (s *Store) CreateOwnershipTransfer(ctx context.Context, saID string, from auth.Principal, toEmail string) (repo.OwnershipTransfer, error) {
	toID, err := s.q.GetUserIDByEmail(ctx, toEmail)
	if errors.Is(err, pgx.ErrNoRows) {
		toID = newID("usr")
		err = s.q.InsertPlaceholderUser(ctx, dbsqlc.InsertPlaceholderUserParams{ID: toID, Email: toEmail})
	}
	if err != nil {
		return repo.OwnershipTransfer{}, err
	}
	id := newID("xfer")
	err = s.q.InsertOwnershipTransfer(ctx, dbsqlc.InsertOwnershipTransferParams{ID: id, ServiceAccountID: saID, FromUserID: from.ID, ToUserID: toID})
	return repo.OwnershipTransfer{ID: id, ServiceAccountID: saID, FromUserID: from.ID, ToUserID: toID, Status: repo.StatusPending}, err
}
func (s *Store) ListOwnershipTransfers(ctx context.Context, user auth.Principal) ([]repo.OwnershipTransfer, error) {
	rows, err := s.q.ListOwnershipTransfers(ctx, user.ID)
	if err != nil {
		return nil, err
	}
	out := make([]repo.OwnershipTransfer, 0, len(rows))
	for _, r := range rows {
		out = append(out, repo.OwnershipTransfer{ID: r.ID, ServiceAccountID: r.ServiceAccountID, FromUserID: r.FromUserID, ToUserID: r.ToUserID, Status: r.Status})
	}
	return out, nil
}
func (s *Store) AcceptOwnershipTransfer(ctx context.Context, id string, user auth.Principal) (repo.OwnershipTransfer, error) {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return repo.OwnershipTransfer{}, err
	}
	defer tx.Rollback(ctx)
	q := s.q.WithTx(tx)
	r, err := q.GetOwnershipTransferForUpdate(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return repo.OwnershipTransfer{}, repo.ErrNotFound
	}
	if err != nil {
		return repo.OwnershipTransfer{}, err
	}
	x := repo.OwnershipTransfer{ID: r.ID, ServiceAccountID: r.ServiceAccountID, FromUserID: r.FromUserID, ToUserID: r.ToUserID, Status: r.Status}
	if x.ToUserID != user.ID {
		return x, repo.ErrNotTransferTarget
	}
	if x.Status != repo.StatusPending {
		return x, repo.ErrTransferNotPending
	}
	if err := q.UpdateServiceAccountOwner(ctx, dbsqlc.UpdateServiceAccountOwnerParams{OwnerUserID: user.ID, ID: x.ServiceAccountID}); err != nil {
		return x, err
	}
	if _, err := q.AcceptOwnershipTransfer(ctx, id); err != nil {
		return x, err
	}
	x.Status = repo.StatusAccepted
	return x, tx.Commit(ctx)
}
func (s *Store) DeclineOwnershipTransfer(ctx context.Context, id string, user auth.Principal) error {
	_, err := s.q.DeclineOwnershipTransfer(ctx, dbsqlc.DeclineOwnershipTransferParams{ID: id, FromUserID: user.ID})
	if errors.Is(err, pgx.ErrNoRows) {
		return repo.ErrNotFound
	}
	return err
}

func docFromGet(r dbsqlc.GetDocumentRow) repo.Document {
	return repo.Document{ID: r.ID, Title: r.Title, CurrentVersionID: r.CurrentVersionID.String, Owner: auth.Principal{Type: auth.PrincipalUser, ID: r.OwnerID, Email: r.OwnerEmail, Name: r.OwnerName}}
}
func docFromList(r dbsqlc.ListDocumentsRow) repo.Document {
	return repo.Document{ID: r.ID, Title: r.Title, CurrentVersionID: r.CurrentVersionID.String, Owner: auth.Principal{Type: auth.PrincipalUser, ID: r.OwnerID, Email: r.OwnerEmail, Name: r.OwnerName}}
}
func commentFromGet(r dbsqlc.GetCommentRow) repo.Comment {
	c := repo.Comment{ID: r.ID, DocumentID: r.DocumentID, VersionID: r.CreatedOnVersionID, Author: auth.Principal{Type: auth.PrincipalType(r.AuthorType), ID: r.AuthorID, Email: r.AuthorEmail, Name: r.AuthorName}, Body: r.Body, SelectedText: r.SelectedText, Status: r.Status}
	_ = json.Unmarshal(r.AnchorJson, &c.Anchor)
	return c
}
func commentFromList(r dbsqlc.ListCommentsRow) repo.Comment {
	c := repo.Comment{ID: r.ID, DocumentID: r.DocumentID, VersionID: r.CreatedOnVersionID, Author: auth.Principal{Type: auth.PrincipalType(r.AuthorType), ID: r.AuthorID, Email: r.AuthorEmail, Name: r.AuthorName}, Body: r.Body, SelectedText: r.SelectedText, Status: r.Status}
	_ = json.Unmarshal(r.AnchorJson, &c.Anchor)
	return c
}
