package repo

import "github.com/anuragrao/aidocs/api/internal/auth"

// NewMemorySeeded returns an in-memory repository preloaded with the canonical
// fixture graph used by the API/server tests: an owner-owned document (doc_1 /
// ver_1), a service account (sa_1), an existing comment (cmt_1), and the role
// conventions for the editor_1/commenter_1/viewer_1 principals.
//
// The roles are seeded into fixtureRoles/fixturePrincipals rather than as real
// grants on purpose: grants would consume the grantN sequence and shift the IDs
// that tests assert on (e.g. the first grant a test creates must be gr_1).
//
// Production code uses NewMemory (empty); only tests should call this.
func NewMemorySeeded() *Memory {
	m := NewMemory()
	owner := auth.Principal{Type: auth.PrincipalUser, ID: "owner_1", Email: "owner@example.com", Name: "Owner"}
	m.users[owner.ID] = owner
	m.docs["doc_1"] = Document{ID: "doc_1", Title: "fixture", Visibility: "private", Owner: owner, CurrentVersionID: "ver_1"}
	m.versions["ver_1"] = Version{ID: "ver_1", Number: 1, DocumentID: "doc_1", CreatedBy: owner, SHA256: "sha_1"}
	m.sas["sa_1"] = ServiceAccount{ID: "sa_1", Name: "fixture", Owner: owner}
	m.comments["cmt_1"] = Comment{ID: "cmt_1", DocumentID: "doc_1", VersionID: "ver_1", Author: auth.Principal{Type: auth.PrincipalUser, ID: "commenter_1", Email: "commenter@example.com", Name: "Commenter"}, Body: "original", Status: StatusOpen}
	m.fixtureRoles = map[string]Role{
		"owner_1":     RoleOwner,
		"editor_1":    RoleEditor,
		"commenter_1": RoleCommenter,
		"viewer_1":    RoleViewer,
	}
	m.fixturePrincipals = map[string]bool{
		"owner_1":     true,
		"editor_1":    true,
		"commenter_1": true,
		"viewer_1":    true,
	}
	return m
}
