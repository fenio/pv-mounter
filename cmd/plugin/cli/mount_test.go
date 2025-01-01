package cli

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"testing"

	"github.com/fenio/pv-mounter/pkg/plugin"
	"github.com/spf13/cobra"
)

// We’ll define a mock function to replace plugin.Mount for testing.
// This way, we can see if it's called with the arguments we expect.
var (
	mockMountFunc        func(namespace, pvcName, localMount string, needsRoot, debug bool) error
	originalPluginMount  = plugin.Mount
)

func TestMain(m *testing.M) {
	// Before running tests, replace the real plugin.Mount with our mockMount wrapper.
	// We'll restore it after tests.
	plugin.Mount = func(ctx interface{}, namespace, pvcName, localMount string, needsRoot, debug bool) error {
		// We only care about the last 5 arguments for testing:
		//   namespace, pvcName, localMount, needsRoot, debug
		// We'll skip the first param (ctx) for simplicity.
		return mockMountFunc(namespace, pvcName, localMount, needsRoot, debug)
	}

	// Run all tests.
	code := m.Run()

	// After tests complete, restore the original plugin.Mount function.
	plugin.Mount = originalPluginMount

	os.Exit(code)
}

func TestMountCmd(t *testing.T) {
	tests := []struct {
		name            string
		args            []string
		envNeedsRoot    string // possible values: "", "true", "false", "invalid"
		envDebug        string // same idea
		flagNeedsRoot   bool
		flagDebug       bool
		expectErrSubstr string // substring of error message we expect, if any
		expectMountCall bool
		expectMountArgs struct {
			namespace       string
			pvcName         string
			localMount      string
			needsRoot, debug bool
		}
	}{
		{
			name:            "Not enough args",
			args:            []string{"default", "my-pvc"}, // only 2 args, need 3
			expectErrSubstr: "requires exactly 3 argument(s)",
			expectMountCall: false,
		},
		{
			name:            "Valid args, no flags or env",
			args:            []string{"default", "my-pvc", "/tmp"},
			expectMountCall: true,
			expectMountArgs: struct {
				namespace       string
				pvcName         string
				localMount      string
				needsRoot, debug bool
			}{
				namespace: "default", pvcName: "my-pvc", localMount: "/tmp", needsRoot: false, debug: false,
			},
		},
		{
			name:            "NeedsRoot flag set",
			args:            []string{"default", "my-pvc", "/tmp"},
			flagNeedsRoot:   true,
			expectMountCall: true,
			expectMountArgs: struct {
				namespace       string
				pvcName         string
				localMount      string
				needsRoot, debug bool
			}{
				namespace: "default", pvcName: "my-pvc", localMount: "/tmp", needsRoot: true, debug: false,
			},
		},
		{
			name:            "Debug env set to true overrides flags",
			args:            []string{"default", "my-pvc", "/tmp"},
			envDebug:        "true",
			flagDebug:       false, // Even if false by flag, env will override to true
			expectMountCall: true,
			expectMountArgs: struct {
				namespace       string
				pvcName         string
				localMount      string
				needsRoot, debug bool
			}{
				namespace: "default", pvcName: "my-pvc", localMount: "/tmp", needsRoot: false, debug: true,
			},
		},
		{
			name:            "Invalid NEEDS_ROOT env triggers error",
			args:            []string{"default", "my-pvc", "/tmp"},
			envNeedsRoot:    "abc", // not a valid bool
			expectErrSubstr: "invalid value for NEEDS_ROOT",
			expectMountCall: false,
		},
		{
			name:            "Mount command returns error",
			args:            []string{"default", "my-pvc", "/tmp"},
			expectErrSubstr: "mock mount error",
			expectMountCall: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup environment variables
			if tt.envNeedsRoot != "" {
				os.Setenv("NEEDS_ROOT", tt.envNeedsRoot)
				defer os.Unsetenv("NEEDS_ROOT")
			} else {
				os.Unsetenv("NEEDS_ROOT")
			}
			if tt.envDebug != "" {
				os.Setenv("DEBUG", tt.envDebug)
				defer os.Unsetenv("DEBUG")
			} else {
				os.Unsetenv("DEBUG")
			}

			// Prepare our mockMountFunc to track calls:
			mockMountFunc = func(namespace, pvcName, localMount string, needsRoot, debug bool) error {
				if tt.expectMountArgs.namespace != "" && namespace != tt.expectMountArgs.namespace {
					t.Errorf("Mount called with namespace=%q; want %q", namespace, tt.expectMountArgs.namespace)
				}
				if tt.expectMountArgs.pvcName != "" && pvcName != tt.expectMountArgs.pvcName {
					t.Errorf("Mount called with pvcName=%q; want %q", pvcName, tt.expectMountArgs.pvcName)
				}
				if tt.expectMountArgs.localMount != "" && localMount != tt.expectMountArgs.localMount {
					t.Errorf("Mount called with localMount=%q; want %q", localMount, tt.expectMountArgs.localMount)
				}
				if needsRoot != tt.expectMountArgs.needsRoot {
					t.Errorf("Mount called with needsRoot=%v; want %v", needsRoot, tt.expectMountArgs.needsRoot)
				}
				if debug != tt.expectMountArgs.debug {
					t.Errorf("Mount called with debug=%v; want %v", debug, tt.expectMountArgs.debug)
				}
				if tt.expectErrSubstr != "" {
					return errors.New("mock mount error") // triggers test for error substring
				}
				return nil
			}

			// Build the command, set flags and args
			cmd := mountCmd()
			cmd.Flags().BoolVar(&tt.flagNeedsRoot, "needs-root", tt.flagNeedsRoot, "")
			cmd.Flags().BoolVar(&tt.flagDebug, "debug", tt.flagDebug, "")

			// Alternatively, you can call `cmd.SetArgs([]string{...})`
			// and rely on the built-in flags. But for demonstration,
			// we manually set them above, then do this:
			cmd.SetArgs(tt.args)

			// Execute
			err := cmd.Execute()

			if err != nil && tt.expectErrSubstr == "" {
				t.Fatalf("Unexpected error: %v", err)
			}
			if err == nil && tt.expectErrSubstr != "" {
				t.Fatalf("Expected error containing %q, got nil", tt.expectErrSubstr)
			}
			if err != nil && tt.expectErrSubstr != "" {
				// Check if error message contains the substring
				if !containsSubstring(err.Error(), tt.expectErrSubstr) {
					t.Errorf("Expected error to contain %q, got %v", tt.expectErrSubstr, err)
				}
			}

			// If we expected no mount call but it was triggered, or vice versa
			// there's no direct "call count" check here, but you can add one by
			// e.g. having a global counter.
			//
			// For demonstration, we only tested the arguments or error from mount.
			// If you want to ensure "no call" was made, you'd set a global bool
			// in mockMountFunc and verify it afterwards.
		})
	}
}

// Utility helper to check if a substring is in a string.
func containsSubstring(str, substr string) bool {
	return len(str) >= len(substr) && (str == substr || (len(str) > len(substr) && (func() bool {
		return (len(str) > 0 && len(substr) > 0 && ( // naive
			str[0:len(substr)] == substr || // at start
			strings.Contains(str, substr))
	})()))
}

