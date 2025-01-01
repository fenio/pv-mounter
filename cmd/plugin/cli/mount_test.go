package cli

import (
    "os"
    "testing"

    "github.com/spf13/cobra"
    "github.com/stretchr/testify/assert"
)

func TestMountCmd(t *testing.T) {
    // Test cases
    tests := []struct {
        name           string
        args           []string
        envNeedsRoot   string
        envDebug       string
        expectedNeedsRoot bool
        expectedDebug  bool
        expectError    bool
    }{
        {
            name:           "NoEnvVars",
            args:           []string{"default", "test-pvc", "/mnt/test"},
            envNeedsRoot:   "",
            envDebug:       "",
            expectedNeedsRoot: false,
            expectedDebug:  false,
            expectError:    false,
        },
        {
            name:           "EnvVarsSet",
            args:           []string{"default", "test-pvc", "/mnt/test"},
            envNeedsRoot:   "true",
            envDebug:       "true",
            expectedNeedsRoot: true,
            expectedDebug:  true,
            expectError:    false,
        },
        {
            name:           "InvalidNeedsRootEnvVar",
            args:           []string{"default", "test-pvc", "/mnt/test"},
            envNeedsRoot:   "invalid",
            envDebug:       "",
            expectedNeedsRoot: false,
            expectedDebug:  false,
            expectError:    true,
        },
        {
            name:           "InvalidDebugEnvVar",
            args:           []string{"default", "test-pvc", "/mnt/test"},
            envNeedsRoot:   "",
            envDebug:       "invalid",
            expectedNeedsRoot: false,
            expectedDebug:  false,
            expectError:    true,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            // Set environment variables
            os.Setenv("NEEDS_ROOT", tt.envNeedsRoot)
            os.Setenv("DEBUG", tt.envDebug)

            // Create the command
            cmd := mountCmd()

            // Set the command arguments
            cmd.SetArgs(tt.args)

            // Run the command and capture any errors
            err := cmd.Execute()

            // Check for expected errors
            if tt.expectError {
                assert.Error(t, err)
            } else {
                assert.NoError(t, err)
            }

            // Cleanup environment variables
            os.Unsetenv("NEEDS_ROOT")
            os.Unsetenv("DEBUG")
        })
    }
}
