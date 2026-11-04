package auth

import "testing"

func TestNormalizeEmail(t *testing.T) {
	if got := NormalizeEmail("  USER@Example.COM  "); got != "user@example.com" {
		t.Fatalf("NormalizeEmail() = %q, want %q", got, "user@example.com")
	}
}

func TestPasswordHashReadsExistingBCrypt(t *testing.T) {
	existingDigest := "$2a$04$IWYUNT5/xfvoyUuPfEiXRu2shBJFdm4r/b30v9wXJWT2UIf1OpdG6"
	if err := VerifyPassword("existing-bcrypt-password", existingDigest); err != nil {
		t.Fatalf("VerifyPassword rejected existing bcrypt digest: %v", err)
	}
	if err := VerifyPassword("not-the-password", existingDigest); err == nil {
		t.Fatal("VerifyPassword accepted a wrong password")
	}
}

func TestHashPasswordProducesCompatibleDigest(t *testing.T) {
	digest, err := HashPassword("new-password")
	if err != nil {
		t.Fatal(err)
	}
	if len(digest) != 60 || digest[:4] != "$2a$" {
		t.Fatalf("HashPassword() = %q, want a 60-character $2a$ bcrypt digest", digest)
	}
	if err := VerifyPassword("new-password", digest); err != nil {
		t.Fatalf("VerifyPassword rejected generated digest: %v", err)
	}
}

func TestSessionCookieSigner(t *testing.T) {
	signer, err := NewSessionSigner([]byte("a-development-key-that-is-at-least-32-bytes"))
	if err != nil {
		t.Fatal(err)
	}

	token := signer.Sign(481)
	if id, ok := signer.Verify(token); !ok || id != 481 {
		t.Fatalf("Verify(Sign(481)) = (%d, %t), want (481, true)", id, ok)
	}
	if _, ok := signer.Verify(token + "x"); ok {
		t.Fatal("Verify accepted a tampered signature")
	}
	if _, ok := signer.Verify("not-a-token"); ok {
		t.Fatal("Verify accepted a malformed token")
	}
}

func TestSessionSignerRequiresLongKey(t *testing.T) {
	if _, err := NewSessionSigner([]byte("short")); err == nil {
		t.Fatal("NewSessionSigner accepted a short key")
	}
}
