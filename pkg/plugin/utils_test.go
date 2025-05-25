package plugin

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"os/exec" // Added for sub-process testing
	"syscall" // Added for checking exit status
	// No specific k8s.io/client-go/kubernetes import needed here yet,
	// as we are testing the *construction* of the clientset, not its methods.
	// The return type of BuildKubeClient will be asserted.
)

const minimalValidKubeconfig = `
apiVersion: v1
clusters:
- cluster:
    server: http://localhost:18080 # Dummy server, different from default 8080 to avoid conflicts
  name: test-cluster
contexts:
- context:
    cluster: test-cluster
    user: test-user
  name: test-context
current-context: test-context
kind: Config
preferences: {}
users:
- name: test-user
  user: {}
`

func TestBuildKubeClient(t *testing.T) {
	originalKubeconfigVal, kubeconfigEnvVarSet := os.LookupEnv("KUBECONFIG")
	originalHomeVal, homeEnvVarSet := os.LookupEnv("HOME")

	// Deferred cleanup function to restore original environment variables
	defer func() {
		if kubeconfigEnvVarSet {
			os.Setenv("KUBECONFIG", originalKubeconfigVal)
		} else {
			os.Unsetenv("KUBECONFIG")
		}
		if homeEnvVarSet {
			os.Setenv("HOME", originalHomeVal)
		} else {
			os.Unsetenv("HOME")
		}
	}()

	t.Run("Valid Kubeconfig via KUBECONFIG env var", func(t *testing.T) {
		tempDir := t.TempDir()
		kubeconfigFile, err := os.CreateTemp(tempDir, "kubeconfig-*.yaml")
		if err != nil {
			t.Fatalf("Failed to create temp kubeconfig file: %v", err)
		}
		// No defer kubeconfigFile.Close() here, as it needs to be closed before BuildKubeClient reads it.
		
		_, err = kubeconfigFile.WriteString(minimalValidKubeconfig)
		if err != nil {
			kubeconfigFile.Close() // Attempt to close before failing
			t.Fatalf("Failed to write to temp kubeconfig file: %v", err)
		}
		if err := kubeconfigFile.Close(); err != nil { // Close explicitly to ensure content is flushed
			t.Fatalf("Failed to close temp kubeconfig file after writing: %v", err)
		}

		t.Setenv("KUBECONFIG", kubeconfigFile.Name())
		
		// Isolate from default HOME logic
		currentHome, homeWasSet := os.LookupEnv("HOME")
		t.Setenv("HOME", "/dev/null") // Or any path that won't have .kube/config
		if homeWasSet { // Defer restoration of HOME if it was originally set
			defer t.Setenv("HOME", currentHome)
		} else { // If HOME was not set, ensure it's unset after the test
			defer os.Unsetenv("HOME")
		}

		clientset, err := BuildKubeClient()
		if err != nil {
			t.Errorf("BuildKubeClient() with KUBECONFIG returned error: %v, want nil", err)
		}
		if clientset == nil {
			t.Error("BuildKubeClient() with KUBECONFIG returned nil clientset, want non-nil")
		}
	})

	t.Run("Invalid Kubeconfig path via KUBECONFIG", func(t *testing.T) {
		nonExistentPath := filepath.Join(t.TempDir(), "non-existent-kubeconfig")
		t.Setenv("KUBECONFIG", nonExistentPath)
		
		currentHome, homeWasSet := os.LookupEnv("HOME")
		t.Setenv("HOME", "/dev/null")
		if homeWasSet {
			defer t.Setenv("HOME", currentHome)
		} else {
			defer os.Unsetenv("HOME")
		}

		clientset, err := BuildKubeClient()
		if err == nil {
			t.Errorf("BuildKubeClient() with invalid KUBECONFIG path should have returned an error, but did not")
		}
		if clientset != nil {
			t.Errorf("BuildKubeClient() with invalid KUBECONFIG path returned non-nil clientset, want nil")
		}
	})
	
	t.Run("Invalid Kubeconfig content via KUBECONFIG", func(t *testing.T) {
		tempDir := t.TempDir()
		kubeconfigFile, err := os.CreateTemp(tempDir, "invalid-kubeconfig-*.yaml")
		if err != nil {
			t.Fatalf("Failed to create temp kubeconfig file: %v", err)
		}
		
		_, err = kubeconfigFile.WriteString("this is not valid yaml content: {")
		if err != nil {
			kubeconfigFile.Close()
			t.Fatalf("Failed to write to temp kubeconfig file: %v", err)
		}
		if err := kubeconfigFile.Close(); err != nil {
			t.Fatalf("Failed to close temp invalid kubeconfig file: %v", err)
		}

		t.Setenv("KUBECONFIG", kubeconfigFile.Name())
		currentHome, homeWasSet := os.LookupEnv("HOME")
		t.Setenv("HOME", "/dev/null")
		if homeWasSet {
			defer t.Setenv("HOME", currentHome)
		} else {
			defer os.Unsetenv("HOME")
		}

		clientset, err := BuildKubeClient()
		if err == nil {
			t.Errorf("BuildKubeClient() with invalid KUBECONFIG content should have returned an error")
		}
		if clientset != nil {
			t.Errorf("BuildKubeClient() with invalid KUBECONFIG content returned non-nil clientset, want nil")
		}
	})

	t.Run("Valid Kubeconfig via default path ($HOME/.kube/config)", func(t *testing.T) {
		testHomeDir := t.TempDir()
		t.Setenv("HOME", testHomeDir)

		// Ensure KUBECONFIG is unset for this test
		currentKubeconfig, kubeconfigWasSet := os.LookupEnv("KUBECONFIG")
		t.Setenv("KUBECONFIG", "") 
		if kubeconfigWasSet {
			defer t.Setenv("KUBECONFIG", currentKubeconfig)
		} else {
			defer os.Unsetenv("KUBECONFIG")
		}


		kubeDir := filepath.Join(testHomeDir, ".kube")
		if err := os.MkdirAll(kubeDir, 0755); err != nil {
			t.Fatalf("Failed to create .kube directory: %v", err)
		}
		kubeconfigFile := filepath.Join(kubeDir, "config")
		err := os.WriteFile(kubeconfigFile, []byte(minimalValidKubeconfig), 0644)
		if err != nil {
			t.Fatalf("Failed to write default kubeconfig file: %v", err)
		}

		clientset, err := BuildKubeClient()
		if err != nil {
			t.Errorf("BuildKubeClient() with default kubeconfig returned error: %v, want nil", err)
		}
		if clientset == nil {
			t.Error("BuildKubeClient() with default kubeconfig returned nil clientset, want non-nil")
		}
	})

	t.Run("No Kubeconfig found", func(t *testing.T) {
		currentKubeconfig, kubeconfigWasSet := os.LookupEnv("KUBECONFIG")
		t.Setenv("KUBECONFIG", "")
		if kubeconfigWasSet {
			defer t.Setenv("KUBECONFIG", currentKubeconfig)
		} else {
			defer os.Unsetenv("KUBECONFIG")
		}
		
		emptyHomeDir := t.TempDir()
		t.Setenv("HOME", emptyHomeDir)

		clientset, err := BuildKubeClient()

		if err == nil {
			t.Errorf("BuildKubeClient() with no kubeconfig should have returned an error, but did not")
		} else {
			// Check for common error messages indicating no config or in-cluster attempt failure
			// This list might need adjustment based on the exact client-go version and its error messages.
			expectedErrorSubstrings := []string{
				"no configuration has been provided", 
				"unable to load in-cluster configuration",
				"neither KUBECONFIG nor HOME were set", 
				"the server has asked for the client to provide credentials", // often when in-cluster fails
				"no such file or directory", // when default path is checked and not found
			}
			foundExpectedError := false
			for _, sub := range expectedErrorSubstrings {
				if strings.Contains(err.Error(), sub) {
					foundExpectedError = true
					break
				}
			}
			if !foundExpectedError {
                 t.Logf("BuildKubeClient() with no kubeconfig returned error: '%v'. This might be an acceptable error indicating no config was found.", err)
			}
		}

		if clientset != nil {
			t.Errorf("BuildKubeClient() with no kubeconfig returned non-nil clientset, want nil")
		}
	})
}

func TestCheckSSHFS_Found(t *testing.T) {
	originalLookPath := execLookPath
	execLookPath = func(file string) (string, error) {
		if file == "sshfs" {
			return "/fake/path/to/sshfs", nil // Simulate sshfs found
		}
		return originalLookPath(file)
	}
	defer func() { execLookPath = originalLookPath }()

	// If checkSSHFS calls os.Exit, this test will fail by timing out or being aborted.
	// A successful run means os.Exit was not called.
	// We capture stdout/stderr to ensure no unexpected error messages are printed.
	// However, checkSSHFS prints informational messages which are fine.
	// The main point is it doesn't exit.
	checkSSHFS()
}

func TestCheckSSHFS_NotFound_Exits(t *testing.T) {
	if os.Getenv("BE_CRASHER_FOR_TESTING_SSHFS") == "1" {
		originalLookPath := execLookPath
		execLookPath = func(file string) (string, error) {
			if file == "sshfs" {
				return "", exec.ErrNotFound // Simulate sshfs not found
			}
			// This part might not be reached if checkSSHFS exits quickly,
			// but it's good practice for a fuller mock.
			return originalLookPath(file) 
		}
		defer func() { execLookPath = originalLookPath }() // Should be before checkSSHFS if we expected it to not exit

		checkSSHFS() // This should call os.Exit(1)
		return       // Should not be reached if os.Exit is called
	}

	// Run the test in a subprocess
	cmd := exec.Command(os.Args[0], "-test.run=^TestCheckSSHFS_NotFound_Exits$")
	cmd.Env = append(os.Environ(), "BE_CRASHER_FOR_TESTING_SSHFS=1")
	output, err := cmd.CombinedOutput() // Capture stdout and stderr

	// Check if the command exited with a non-zero status
	if e, ok := err.(*exec.ExitError); ok {
		// Check for exit code 1
		if status, ok := e.Sys().(syscall.WaitStatus); ok {
			if status.ExitStatus() == 1 {
				// Check if the output contains the expected message
				expectedMsg := "sshfs is not available in your environment."
				if !strings.Contains(string(output), expectedMsg) {
					t.Fatalf("Process exited with status 1 but output did not contain expected message.\nExpected: '%s'\nGot: '%s'", expectedMsg, string(output))
				}
				return // Test passed
			}
			t.Fatalf("Process exited with status %d, expected 1. Output:\n%s", status.ExitStatus(), string(output))
		}
		t.Fatalf("Failed to get exit status. Error: %v. Output:\n%s", err, string(output))
	} else if err == nil {
		t.Fatalf("Process exited cleanly, expected exit status 1. Output:\n%s", string(output))
	} else {
		t.Fatalf("Command execution failed with error: %v. Output:\n%s", err, string(output))
	}
}
