package cli

import (
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/fenio/pv-mounter/pkg/plugin"
	"github.com/spf13/cobra"
)

// mockMountFunc is our test replacement for plugin.Mount().
// We'll track calls in the tests below.
var (
	mockMountFunc       func(namespace, pvcName, localMount string, needsRoot, debug bool) error
	originalPluginMount = plugin.Mount
)

// TestMain runs before any test in this file. We'll swap out plugin.Mount with
// our mock, run tests, then restore the original function.
func TestMain(m *testing.M) {
	plugin.Mount = func(ctx interface{}, namespace, pvcName, localMount string, needsRoot, debug bool) error {
		return mockMountFunc(namespace, pvcName, localMount, needsRoot, debug)
	}

	code := m.Run()

	// Restore the original plugin.Mount after tests complete.
	plugin.Mount = originalPluginMount

	os.Exit(code)
}

func TestMountCmd(t *testing.T) {
	tests := []struct {
		name            string
		args            []string
		envNeedsRoot    string // e.g. "", "true", "false", or invalid
		envDebug        string
		flagNeedsRoot   bool
		flagDebug       bool
		expectErrSubstr string
		expectMountArgs struct {
			namespace  string
			pvcName    string
			localMount string
			needsRoot  bool
			debug      bool
		}
		expectMountCall bool
	}{
		{
			name:            "Not enough args",
			args:            []string{"default", "my-pvc"}, // only 2 instead of 3
			expectErrSubstr: "requires exactly 3 argument(s)",
			expectMountCall: false,
		},
		{
			name:            "Valid args, no flags or env",
			args:            []string{"default", "my-pvc", "/tmp"},
			expectMountCall: true,
			expectMountArgs: struct {
				namespace  string
				pvcName    string
				localMount string
				needsRoot  bool
				debug      bool
			}{
				namespace: "default", pvcName: "my-pvc", localMount: "/tmp",
				needsRoot: false, debug: false,
			},
		},
		{
			name:          "NeedsRoot flag set",
			args:          []string{"default", "my-pvc", "/tmp"},
			flagNeedsRoot: true,
			expectMountCall: true,
			expectMountArgs: struct {
				namespace  string
				pvcName    string
				localMount string
				needsRoot  bool
				debug      bool
			}{
				namespace: "default", pvcName: "my-pvc", localMount: "/tmp",
				needsRoot: true, debug: false,
			},
		},
		{
			name:      "Debug env set to true overrides flag",
			args:      []string{"default", "my-pvc", "/tmp"},
			envDebug:  "true",
			flagDebug: false, // env should override to true
			expectMountCall: true,
			expectMountArgs: struct {
				namespace  string
				pvcName    string
				localMount string
				needsRoot  bool
				debug      bool
			}{
				namespace: "default", pvcName: "my-pvc", localMount: "/tmp",
				needsRoot: false, debug: true,
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
			// Setup env vars if specified
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

			// Reset or define our mockMountFunc for each test
			mockMountFunc = func(namespace, pvcName, localMount string, needsRoot, debug bool) error {
				// Check if the user expected plugin.Mount to be called or not
				// We'll rely on expectMountCall to see if they wanted this called at all.
				if tt.expectMountCall {
					// Validate the arguments if they've been set
					if tt.expectMountArgs.namespace != "" && tt.expectMountArgs.namespace != namespace {
						t.Errorf("Mount called with namespace=%q; want %q", namespace, tt.expectMountArgs.namespace)
					}
					if tt.expectMountArgs.pvcName != "" && tt.expectMountArgs.pvcName != pvcName {
						t.Errorf("Mount called with pvcName=%q; want %q", pvcName, tt.expectMountArgs.pvcName)
					}
					if tt.expectMountArgs.localMount != "" && tt.expectMountArgs.localMount != localMount {
						t.Errorf("Mount called with localMount=%q; want %q", localMount, tt.expectMountArgs.localMount)
					}
					if needsRoot != tt.expectMountArgs.needsRoot {
						t.Errorf("Mount called with needsRoot=%v; want %v", needsRoot, tt.expectMountArgs.needsRoot)
					}
					if debug != tt.expectMountArgs.debug {
						t.Errorf("Mount called with debug=%v; want %v", debug, tt.expectMountArgs.debug)
					}
					// Simulate an error if the test expects one
					if tt.expectErrSubstr != "" {
						return errors.New("mock mount error")
					}
				} else {
					// If we did NOT expect a call, but it happened, that's an error
					t.Errorf("plugin.Mount was called unexpectedly!")
				}

				return nil
			}

			// Build the mount command
			cmd := mountCmd()

			// Manually set the flags to simulate user input
			cmd.Flags().Bool("needs-root", tt.flagNeedsRoot, "")
			cmd.Flags().Bool("debug", tt.flagDebug, "")
			// Alternatively, cmd.SetArgs can parse the flags out of the slice
			// but if we do it that way, we'd do something like:
			// cmd.SetArgs(append(tt.args, "--needs-root="+strconv.FormatBool(tt.flagNeedsRoot)))
			// For simplicity, let's keep these two separate.

			// Now set the positional args
			cmd.SetArgs(tt.args)

			err := cmd.Execute()
			if err != nil && tt.expectErrSubstr == "" {
				t.Fatalf("Unexpected error: %v", err)
			}
			if err == nil && tt.expectErrSubstr != "" {
				t.Fatalf("Expected error containing %q, got nil", tt.expectErrSubstr)
			}
			if err != nil && tt.expectErrSubstr != "" {
				// Check if error message contains the substring
				if !strings.Contains(err.Error(), tt.expectErrSubstr) {
					t.Errorf("Expected error to contain %q, got %v", tt.expectErrSubstr, err)
				}
			}
		})
	}
}

// Helper for checking substring
// (We can just do strings.Contains in-line, but let's keep a helper.)
func containsSubstring(haystack, needle string) bool {
	return strings.Contains(haystack, needle)
}

