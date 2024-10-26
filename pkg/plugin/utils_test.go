package plugin

import (
	// Necessary imports
	"crypto/elliptic"
	"strings"
	"testing"
)

func TestRandSeq(t *testing.T) {
	length := 10
	seq := randSeq(length)
	if len(seq) != length {
		t.Errorf("Expected length %d, got %d", length, len(seq))
	}
	allowedChars := "abcdefghijklmnopqrstuvwxyz0123456789"
	for _, char := range seq {
		if !strings.ContainsRune(allowedChars, char) {
			t.Errorf("Invalid character %c in sequence", char)
		}
	}
}

func TestGenerateKeyPair(t *testing.T) {
	privateKey, publicKey, err := GenerateKeyPair(elliptic.P256())
	if err != nil {
		t.Fatalf("GenerateKeyPair returned an error: %v", err)
	}
	if privateKey == "" || publicKey == "" {
		t.Error("GenerateKeyPair returned empty keys")
	}
	// Additional checks can be added to validate the key formats
}
