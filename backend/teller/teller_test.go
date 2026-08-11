package teller

import (
	"bytes"
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

func TestAESGCMDecryptValidatesStoredTokenData(t *testing.T) {
	cipher, err := NewAESGCM([]byte("0123456789abcdef0123456789abcdef"))
	if err != nil {
		t.Fatal(err)
	}
	ciphertext, nonce, err := cipher.Encrypt([]byte("access-token"))
	if err != nil {
		t.Fatal(err)
	}
	plaintext, err := cipher.Decrypt(ciphertext, nonce)
	if err != nil || !bytes.Equal(plaintext, []byte("access-token")) {
		t.Fatalf("Decrypt() = %q, %v", plaintext, err)
	}
	if _, err := cipher.Decrypt(ciphertext, nil); err == nil {
		t.Fatal("Decrypt() accepted an invalid nonce")
	}
	if _, err := cipher.Decrypt(nil, nonce); err == nil {
		t.Fatal("Decrypt() accepted an invalid ciphertext")
	}
}
