// Package bots generates and validates the email-shaped addresses
// used as the public identity of service accounts (e.g. n8n-prod@brave.otter.bot).
package bots

import (
	"crypto/rand"
	"errors"
	"fmt"
	"math/big"
	"regexp"
	"strings"
)

const tld = ".bot"

var (
	labelRE  = regexp.MustCompile(`^[a-z0-9](?:[a-z0-9-]{0,29}[a-z0-9])?$`)
	domainRE = regexp.MustCompile(`^[a-z0-9](?:[a-z0-9-]*[a-z0-9])?(?:\.[a-z0-9](?:[a-z0-9-]*[a-z0-9])?)*$`)
)

// ErrInvalidLabel is returned when the local part fails validation.
var ErrInvalidLabel = errors.New("invalid_label")

// ErrInvalidDomain is returned when the domain fails validation or doesn't end in .bot.
var ErrInvalidDomain = errors.New("invalid_domain")

// ValidateLabel checks the part before the @. Letters, numbers, hyphens; 1-31 chars.
func ValidateLabel(label string) error {
	if !labelRE.MatchString(label) {
		return ErrInvalidLabel
	}
	return nil
}

// ValidateDomain checks the part after the @. Must end in .bot, max 63 chars,
// labels separated by dots, lowercase alphanumeric plus hyphens.
func ValidateDomain(domain string) error {
	if !strings.HasSuffix(domain, tld) {
		return ErrInvalidDomain
	}
	if len(domain) > 63 || len(domain) <= len(tld) {
		return ErrInvalidDomain
	}
	if !domainRE.MatchString(domain) {
		return ErrInvalidDomain
	}
	return nil
}

// Compose joins label and domain into a full bot address. Caller must have
// validated both halves first.
func Compose(label, domain string) string { return label + "@" + domain }

// IsBotAddress reports whether s is a syntactically valid bot address.
func IsBotAddress(s string) bool {
	label, domain, ok := Split(s)
	if !ok {
		return false
	}
	return ValidateLabel(label) == nil && ValidateDomain(domain) == nil
}

// Split breaks an address into (label, domain). The boolean is false if there's
// no exactly-one '@' or either side is empty.
func Split(s string) (label, domain string, ok bool) {
	parts := strings.Split(s, "@")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", false
	}
	return parts[0], parts[1], true
}

// GenerateDomain returns a randomly chosen `<adjective>.<noun>.bot` string.
// Uses crypto/rand for each word so callers can retry on collision.
func GenerateDomain() string {
	return fmt.Sprintf("%s.%s%s", pick(adjectives), pick(nouns), tld)
}

// GenerateDomainExtended returns a 3-word fallback `<adjective>.<noun>.<adjective>.bot`
// for the (extremely rare) case where retries with two words all collide.
func GenerateDomainExtended() string {
	return fmt.Sprintf("%s.%s.%s%s", pick(adjectives), pick(nouns), pick(adjectives), tld)
}

func pick(xs []string) string {
	n, err := rand.Int(rand.Reader, big.NewInt(int64(len(xs))))
	if err != nil {
		return xs[0]
	}
	return xs[n.Int64()]
}
