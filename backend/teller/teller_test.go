package teller

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"strings"
	"testing"
)

func TestEd25519EnrollmentVerifier(t *testing.T) {
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	verifier, err := NewEd25519EnrollmentVerifier(base64.StdEncoding.EncodeToString(publicKey))
	if err != nil {
		t.Fatal(err)
	}
	values := []string{"nonce", "token", "usr_1", "enr_1", "sandbox"}
	signature := base64.StdEncoding.EncodeToString(ed25519.Sign(privateKey, []byte(strings.Join(values, "."))))
	if err := verifier.Verify(values[0], values[1], values[2], values[3], values[4], []string{signature}); err != nil {
		t.Fatalf("Verify() error = %v", err)
	}
	if err := verifier.Verify("other-nonce", values[1], values[2], values[3], values[4], []string{signature}); err == nil {
		t.Fatal("Verify() accepted a signature for a different nonce")
	}
}
