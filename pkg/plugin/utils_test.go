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

func TestGenerateKeyPair_DifferentCurves(t *testing.T) {
	curves := []elliptic.Curve{
		elliptic.P256(),
		elliptic.P384(),
		elliptic.P521(),
	}

	for _, curve := range curves {
		t.Run(curve.Params().Name, func(t *testing.T) {
			privateKey, publicKey, err := GenerateKeyPair(curve)
			if err != nil {
				t.Fatalf("GenerateKeyPair with curve %s returned an error: %v", curve.Params().Name, err)
			}
			if privateKey == "" {
				t.Error("GenerateKeyPair returned empty private key")
			}
			if publicKey == "" {
				t.Error("GenerateKeyPair returned empty public key")
			}
			if !strings.Contains(privateKey, "BEGIN EC PRIVATE KEY") {
				t.Error("Private key does not contain expected PEM header")
			}
			if !strings.HasPrefix(publicKey, "ecdsa-sha2-") {
				t.Error("Public key does not have expected SSH format")
			}
		})
	}
}

func TestGenerateKeyPair_NilCurve(t *testing.T) {
	_, _, err := GenerateKeyPair(nil)
	if err == nil {
		t.Error("Expected error when curve is nil")
	}
	if err != nil && !strings.Contains(err.Error(), "curve must not be nil") {
		t.Errorf("Expected 'curve must not be nil' error, got: %v", err)
	}
}

func TestRandSeq_Uniqueness(t *testing.T) {
	length := 10
	iterations := 100
	seen := make(map[string]bool)

	for i := 0; i < iterations; i++ {
		seq := randSeq(length)
		if seen[seq] {
			t.Logf("Warning: duplicate sequence generated: %s (this is unlikely but possible)", seq)
		}
		seen[seq] = true
	}

	if len(seen) < iterations/2 {
		t.Errorf("Expected at least %d unique sequences, got %d", iterations/2, len(seen))
	}
}

func TestValidateKubernetesName(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		fieldName string
		wantErr   bool
	}{
		{
			name:      "valid name",
			input:     "my-pvc",
			fieldName: "pvc-name",
			wantErr:   false,
		},
		{
			name:      "valid name with numbers",
			input:     "pvc-123",
			fieldName: "pvc-name",
			wantErr:   false,
		},
		{
			name:      "valid single character",
			input:     "a",
			fieldName: "namespace",
			wantErr:   false,
		},
		{
			name:      "empty name",
			input:     "",
			fieldName: "namespace",
			wantErr:   true,
		},
		{
			name:      "name too long",
			input:     strings.Repeat("a", 254),
			fieldName: "pvc-name",
			wantErr:   true,
		},
		{
			name:      "uppercase letters",
			input:     "MyPVC",
			fieldName: "pvc-name",
			wantErr:   true,
		},
		{
			name:      "starts with hyphen",
			input:     "-invalid",
			fieldName: "namespace",
			wantErr:   true,
		},
		{
			name:      "ends with hyphen",
			input:     "invalid-",
			fieldName: "pvc-name",
			wantErr:   true,
		},
		{
			name:      "contains underscore",
			input:     "my_pvc",
			fieldName: "pvc-name",
			wantErr:   true,
		},
		{
			name:      "contains dot",
			input:     "my.pvc",
			fieldName: "pvc-name",
			wantErr:   false,
		},
		{
			name:      "contains multiple dots",
			input:     "ixx-blueapi-scratch-1.0.0",
			fieldName: "pvc-name",
			wantErr:   false,
		},
		{
			name:      "starts with dot",
			input:     ".invalid",
			fieldName: "pvc-name",
			wantErr:   true,
		},
		{
			name:      "ends with dot",
			input:     "invalid.",
			fieldName: "pvc-name",
			wantErr:   true,
		},
		{
			name:      "valid at max length",
			input:     strings.Repeat("a", 253),
			fieldName: "namespace",
			wantErr:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateKubernetesName(tt.input, tt.fieldName)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateKubernetesName() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
