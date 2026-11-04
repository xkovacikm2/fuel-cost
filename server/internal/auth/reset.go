package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

type PasswordResetSigner struct {
	key []byte
}

func NewPasswordResetSigner(key []byte) (*PasswordResetSigner, error) {
	if len(key) < 32 {
		return nil, errors.New("password reset signing key must be at least 32 bytes")
	}
	return &PasswordResetSigner{key: append([]byte(nil), key...)}, nil
}

func (s *PasswordResetSigner) Sign(userID int64, passwordDigest string, now time.Time) string {
	fingerprint := sha256.Sum256([]byte(passwordDigest))
	payload := fmt.Sprintf("%d:%d:%x", userID, now.Add(15*time.Minute).Unix(), fingerprint)
	encodedPayload := base64.RawURLEncoding.EncodeToString([]byte(payload))
	return encodedPayload + "." + s.signature(encodedPayload)
}

func (s *PasswordResetSigner) Verify(token, passwordDigest string, now time.Time) (int64, bool) {
	encodedPayload, signature, ok := strings.Cut(token, ".")
	if !ok || encodedPayload == "" || signature != s.signature(encodedPayload) {
		return 0, false
	}
	decoded, err := base64.RawURLEncoding.DecodeString(encodedPayload)
	if err != nil {
		return 0, false
	}
	parts := strings.Split(string(decoded), ":")
	if len(parts) != 3 {
		return 0, false
	}
	userID, idErr := strconv.ParseInt(parts[0], 10, 64)
	expiresAt, expiryErr := strconv.ParseInt(parts[1], 10, 64)
	fingerprint := sha256.Sum256([]byte(passwordDigest))
	if idErr != nil || expiryErr != nil || userID < 1 || now.Unix() > expiresAt || !hmac.Equal([]byte(parts[2]), []byte(fmt.Sprintf("%x", fingerprint))) {
		return 0, false
	}
	return userID, true
}

func (s *PasswordResetSigner) UserID(token string) (int64, bool) {
	encodedPayload, _, ok := strings.Cut(token, ".")
	if !ok || encodedPayload == "" {
		return 0, false
	}
	decoded, err := base64.RawURLEncoding.DecodeString(encodedPayload)
	if err != nil {
		return 0, false
	}
	parts := strings.SplitN(string(decoded), ":", 2)
	if len(parts) != 2 {
		return 0, false
	}
	userID, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil || userID < 1 {
		return 0, false
	}
	return userID, true
}

func (s *PasswordResetSigner) signature(payload string) string {
	mac := hmac.New(sha256.New, s.key)
	_, _ = mac.Write([]byte(payload))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}
