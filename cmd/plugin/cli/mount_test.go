package cli

import (
	"os"
	"strings"
	"testing"
)

// TestMountCmd_NumArgs checks that Cobra enforces exactly 3 positional args.
func TestMountCmd_NumArgs(t *testing.T) {
	cmd := mountCmd()

	// Test: zero arguments
	cmd.SetArgs([]string{})
	err := cmd.Execute()
	if err == nil {
		t.Fatalf("Expected an error with 0 args, got nil")
	}
	if !strings.Contains(err.Error(), "requires exactly 3 argument(s)") {
		t.Errorf("Expected usage error about 3 arguments, got: %v", err)
	}

	// Test: too many arguments (4)
	cmd = mountCmd()
	cmd.SetArgs([]string{"default", "pvc", "/mnt", "extra-arg"})
	err = cmd.Execute()
	if err == nil {
		t.Fatalf("Expected an error with 4 args, got nil")
	}
	if !strings.Contains(err.Error(), "requires exactly 3 argument(s)") {
		t.Errorf("Expected usage error about 3 arguments, got: %v", err)
	}

	// Test: correct number of arguments (3)
	cmd = mountCmd()
	cmd.SetArgs([]string{"default", "pvc", "/mnt"})
	err = cmd.Execute()
	// NOTE: This might fail if plugin.Mount actually tries to run real logic (kubectl, etc.).
	// But at least it won’t fail on usage arguments. If plugin.Mount fails,
	// you'll get that error instead—but this confirms the CLI recognized 3 args.
	if err != nil && strings.Contains(err.Error(), "requires exactly 3 argument(s)") {
		t.Errorf("Unexpected usage error with 3 args: %v", err)
	}
}

// TestMountCmd_InvalidEnv checks that an invalid NEEDS_ROOT environment var triggers an error.
func TestMountCmd_InvalidEnv(t *testing.T) {
	// Set NEEDS_ROOT to an invalid boolean
	os.Setenv("NEEDS_ROOT", "abc")
	defer os.Unsetenv("NEEDS_ROOT")

	cmd := mountCmd()
	cmd.SetArgs([]string{"default", "pvc", "/mnt"})

	err := cmd.Execute()
	if err == nil {
		t.Fatalf("Expected an error about invalid NEEDS_ROOT, got nil")
	}
	if !strings.Contains(err.Error(), "invalid value for NEEDS_ROOT") {
		t.Errorf("Expected error about invalid NEEDS_ROOT, got: %v", err)
	}
}

// TestMountCmd_ValidEnv checks that valid boolean env vars don’t produce usage errors.
func TestMountCmd_ValidEnv(t *testing.T) {
	os.Setenv("NEEDS_ROOT", "true")
	defer os.Unsetenv("NEEDS_ROOT")

	os.Setenv("DEBUG", "false")
	defer os.Unsetenv("DEBUG")

	cmd := mountCmd()
	cmd.SetArgs([]string{"default", "pvc", "/mnt"})
	err := cmd.Execute()
	// If plugin.Mount tries to run real logic, you might get a different error,
	// but you should NOT get an error about invalid NEEDS_ROOT or invalid DEBUG.
	if err != nil && strings.Contains(err.Error(), "invalid value for NEEDS_ROOT") {
		t.Errorf("Unexpected invalid value error for NEEDS_ROOT: %v", err)
	}
	if err != nil && strings.Contains(err.Error(), "invalid value for DEBUG") {
		t.Errorf("Unexpected invalid value error for DEBUG: %v", err)
	}
}

