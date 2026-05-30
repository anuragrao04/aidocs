package server

import (
	"net/url"
	"strings"

	"github.com/anuragrao/aidocs/api/internal/repo"
)

var roleRank = map[repo.Role]int{repo.RoleViewer: 1, repo.RoleCommenter: 2, repo.RoleEditor: 3, repo.RoleOwner: 4}

func atLeast(have repo.Role, need repo.Role) bool {
	return roleRank[have] >= roleRank[need]
}

func safeWebRedirect(redirect string) bool {
	if redirect == "" {
		return true
	}
	if strings.HasPrefix(redirect, "/") && !strings.HasPrefix(redirect, "//") {
		return true
	}
	u, err := url.Parse(redirect)
	return err == nil && u.Scheme == "" && u.Host == ""
}

func validGrantRole(role repo.Role) bool {
	return role == repo.RoleViewer || role == repo.RoleCommenter || role == repo.RoleEditor
}

func hostMatchesOrigin(host, origin string) bool {
	u, err := url.Parse(origin)
	if err != nil || u.Host == "" {
		return false
	}
	return strings.EqualFold(host, u.Host)
}

func emailDomainAllowed(email string, allowed []string) bool {
	if len(allowed) == 0 {
		return true
	}
	at := strings.LastIndex(email, "@")
	if at < 0 || at == len(email)-1 {
		return false
	}
	domain := strings.ToLower(email[at+1:])
	for _, d := range allowed {
		if strings.EqualFold(domain, strings.TrimSpace(d)) {
			return true
		}
	}
	return false
}
