package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"strconv"
	"strings"

	"golang.org/x/crypto/bcrypt"
)

type SessionSigner struct {
	key []byte
}

func NormalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

func HashPassword(password string) (string, error) {
	digest, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(digest), nil
}

func VerifyPassword(password, digest string) error {
	return bcrypt.CompareHashAndPassword([]byte(digest), []byte(password))
}

func NewSessionSigner(key []byte) (*SessionSigner, error) {
	if len(key) < 32 {
		return nil, strconv.ErrSyntax
	}
	return &SessionSigner{key: append([]byte(nil), key...)}, nil
}

func (s *SessionSigner) Sign(sessionID int64) string {
	payload := base64.RawURLEncoding.EncodeToString([]byte(strconv.FormatInt(sessionID, 10)))
	mac := hmac.New(sha256.New, s.key)
	_, _ = mac.Write([]byte(payload))
	signature := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	return payload + "." + signature
}

func (s *SessionSigner) Verify(token string) (int64, bool) {
	payload, encodedSignature, ok := strings.Cut(token, ".")
	if !ok || payload == "" || encodedSignature == "" || strings.Contains(encodedSignature, ".") {
		return 0, false
	}
	signature, err := base64.RawURLEncoding.DecodeString(encodedSignature)
	if err != nil {
		return 0, false
	}
	mac := hmac.New(sha256.New, s.key)
	_, _ = mac.Write([]byte(payload))
	if !hmac.Equal(signature, mac.Sum(nil)) {
		return 0, false
	}
	decoded, err := base64.RawURLEncoding.DecodeString(payload)
	if err != nil {
		return 0, false
	}
	sessionID, err := strconv.ParseInt(string(decoded), 10, 64)
	if err != nil || sessionID <= 0 {
		return 0, false
	}
	return sessionID, true
}
