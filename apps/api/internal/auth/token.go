package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
)

func NewRandomTokenSource() TokenSource {
	return func() (string, string, error) {
		authToken, err := randomToken()
		if err != nil {
			return "", "", err
		}
		csrfToken, err := randomToken()
		if err != nil {
			return "", "", err
		}
		return authToken, csrfToken, nil
	}
}

func HashToken(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

func randomToken() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}
