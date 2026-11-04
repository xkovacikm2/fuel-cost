package auth

import (
	"testing"
	"time"
)

func TestPasswordResetTokenExpiresAndIsBoundToCurrentPassword(t *testing.T) {
	signer, err := NewPasswordResetSigner([]byte("password-reset-signing-key-that-is-at-least-32-bytes"))
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 11, 3, 10, 0, 0, 0, time.UTC)
	token := signer.Sign(82, "$2a$existing-digest", now)
	if id, ok := signer.Verify(token, "$2a$existing-digest", now.Add(14*time.Minute)); !ok || id != 82 {
		t.Fatalf("valid reset token = (%d, %t), want (82, true)", id, ok)
	}
	if _, ok := signer.Verify(token, "$2a$changed-digest", now.Add(time.Minute)); ok {
		t.Fatal("reset token remained valid after password changed")
	}
	if _, ok := signer.Verify(token, "$2a$existing-digest", now.Add(16*time.Minute)); ok {
		t.Fatal("expired reset token was accepted")
	}
	if _, ok := signer.Verify("forged-token", "$2a$existing-digest", now); ok {
		t.Fatal("forged reset token was accepted")
	}
}
