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

func TestGenerateKeyPair_InvalidCurve(t *testing.T) {
	// Pass nil as the curve, which should cause GenerateKeyPair to fail
	privateKey, publicKey, err := GenerateKeyPair(nil)
	if err == nil {
		t.Error("Expected error when passing nil curve, got nil")
	}
	if privateKey != "" || publicKey != "" {
		t.Error("Expected empty keys when error occurs")
	}
}

func TestRandSeq_ZeroAndNegativeLength(t *testing.T) {
	if seq := randSeq(0); seq != "" {
		t.Errorf("Expected empty string for zero length, got %q", seq)
	}
	// Negative length is not possible for int, but let's check behavior for -1 cast to uint
	if seq := randSeq(int(-1)); seq != "" {
		t.Errorf("Expected empty string for negative length, got %q", seq)
	}
}
