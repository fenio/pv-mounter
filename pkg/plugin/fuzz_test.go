package plugin

import (
    "crypto/elliptic"
    "strings"
    "testing"
)

// FuzzRandSeq fuzzes randSeq ensuring only allowed characters are produced
// and the returned string length matches the requested length for n > 0.
func FuzzRandSeq(f *testing.F) {
    // Seed corpus
    f.Add(int(0))
    f.Add(int(1))
    f.Add(int(5))
    f.Add(int(10))
    f.Add(int(100))

    allowedChars := "abcdefghijklmnopqrstuvwxyz0123456789"

    f.Fuzz(func(t *testing.T, n int) {
        // Avoid huge allocations; clamp to a reasonable size
        if n > 2000 {
            n = n % 2000
        }
        if n < -2000 {
            n = -2000
        }

        s := randSeq(n)

        if n <= 0 {
            if s != "" {
                t.Fatalf("expected empty string for n <= 0; got %q (n=%d)", s, n)
            }
            return
        }

        if len(s) != n {
            t.Fatalf("length mismatch: want %d, got %d", n, len(s))
        }
        for _, r := range s {
            if !strings.ContainsRune(allowedChars, r) {
                t.Fatalf("invalid character %q in output", r)
            }
        }
    })
}

// FuzzGenerateKeyPair fuzzes curve selection for key generation.
func FuzzGenerateKeyPair(f *testing.F) {
    // Seed corpus: include a few selector bytes
    f.Add(byte(0))
    f.Add(byte(1))
    f.Add(byte(2))
    f.Add(byte(3))
    f.Add(byte(4))

    selectCurve := func(sel byte) elliptic.Curve {
        switch sel % 4 {
        case 0:
            return nil
        case 1:
            return elliptic.P256()
        case 2:
            return elliptic.P384()
        default:
            return elliptic.P521()
        }
    }

    f.Fuzz(func(t *testing.T, sel byte) {
        curve := selectCurve(sel)
        priv, pub, err := GenerateKeyPair(curve)

        if curve == nil {
            if err == nil {
                t.Fatalf("expected error for nil curve, got nil")
            }
            if priv != "" || pub != "" {
                t.Fatalf("expected empty keys on error; got priv %d bytes, pub %d bytes", len(priv), len(pub))
            }
            return
        }

        if err != nil {
            t.Fatalf("unexpected error for valid curve: %v", err)
        }
        if priv == "" || pub == "" {
            t.Fatalf("expected non-empty keys for valid curve")
        }
        // Public key should not contain whitespace at the ends
        if strings.TrimSpace(pub) != pub {
            t.Fatalf("public key contains leading/trailing whitespace")
        }
    })
}

// FuzzGeneratePodNameAndPort fuzzes role input ensuring constraints on output.
func FuzzGeneratePodNameAndPort(f *testing.F) {
    // Seed corpus
    f.Add("")
    f.Add("proxy")
    f.Add("standalone")
    f.Add("weird-role")

    f.Fuzz(func(t *testing.T, role string) {
        name, port := generatePodNameAndPort(role)

        if port < 1024 || port > 65535 {
            t.Fatalf("port out of range: %d", port)
        }
        if name == "" {
            t.Fatalf("empty pod name")
        }
        // Check prefix depending on role
        if role == "proxy" {
            if !strings.HasPrefix(name, "volume-exposer-proxy-") {
                t.Fatalf("expected proxy prefix; got %q", name)
            }
        } else {
            if !strings.HasPrefix(name, "volume-exposer-") {
                t.Fatalf("expected default prefix; got %q", name)
            }
        }
    })
}