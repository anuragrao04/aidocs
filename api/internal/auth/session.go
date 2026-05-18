package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"strings"
	"time"
)

type SessionCodec struct{ Secret []byte }

type signedPayload struct {
	Subject   string `json:"sub"`
	Audience  string `json:"aud"`
	ExpiresAt int64  `json:"exp"`
}

func (s SessionCodec) Sign(userID string) string {
	return s.SignForAudience(userID, "session", 30*24*time.Hour)
}

func (s SessionCodec) SignFor(userID string, ttl time.Duration) string {
	return s.SignForAudience(userID, "generic", ttl)
}

func (s SessionCodec) SignForAudience(userID, audience string, ttl time.Duration) string {
	p := signedPayload{Subject: userID, Audience: audience, ExpiresAt: time.Now().Add(ttl).Unix()}
	payload, _ := json.Marshal(p)
	payloadB64 := base64.RawURLEncoding.EncodeToString(payload)
	mac := hmac.New(sha256.New, s.Secret)
	mac.Write([]byte(payloadB64))
	sig := mac.Sum(nil)
	return "v1." + payloadB64 + "." + base64.RawURLEncoding.EncodeToString(sig)
}
func (s SessionCodec) Verify(v string) (string, bool) {
	return s.VerifyAudience(v, "generic")
}

func (s SessionCodec) VerifySession(v string) (string, bool) {
	return s.VerifyAudience(v, "session")
}

func (s SessionCodec) VerifyAudience(v, audience string) (string, bool) {
	parts := strings.Split(v, ".")
	if len(parts) != 3 || parts[0] != "v1" {
		return "", false
	}
	sig, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return "", false
	}
	mac := hmac.New(sha256.New, s.Secret)
	mac.Write([]byte(parts[1]))
	if !hmac.Equal(sig, mac.Sum(nil)) {
		return "", false
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return "", false
	}
	var p signedPayload
	if err := json.Unmarshal(payload, &p); err != nil || p.Subject == "" || p.Audience != audience || p.ExpiresAt < time.Now().Unix() {
		return "", false
	}
	return p.Subject, true
}
