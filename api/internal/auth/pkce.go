package auth

import (
	"crypto/sha256"
	"encoding/base64"
)

func VerifyPKCES256(verifier, challenge string) bool {
	s := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(s[:]) == challenge
}
