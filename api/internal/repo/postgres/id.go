package postgres

import (
	"crypto/rand"
	"encoding/base32"
	"strings"
)

func newID(prefix string) string {
	var b [10]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic(err)
	}
	enc := base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(b[:])
	return prefix + "_" + strings.ToLower(enc)
}
