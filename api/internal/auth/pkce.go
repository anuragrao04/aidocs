package auth

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
)

func VerifyPKCES256(verifier, challenge string) bool {
	s := sha256.Sum256([]byte(verifier))
	// challenge is the base64url-encoded SHA-256 of the verifier. Decode it and
	// compare the raw digest bytes in constant time, matching the
	// subtle.ConstantTimeCompare style used in session.go.
	want, err := base64.RawURLEncoding.DecodeString(challenge)
	if err != nil {
		return false
	}
	return subtle.ConstantTimeCompare(s[:], want) == 1
}
