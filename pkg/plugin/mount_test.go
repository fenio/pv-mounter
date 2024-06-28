package plugin

import (
	"testing"
	"os/exec"
	"fmt"
	"path/filepath"
)

func runCommand(cmdStr string) error {
	cmd := exec.Command("sh", "-c", cmdStr)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("command '%s' failed with error: %v and output: %s", cmdStr, err, string(output))
	}
	return nil
}

func TestMountPVC(t *testing.T) {
	// Get the absolute path to the test directory
	testDir, err := filepath.Abs("test")
	if err != nil {
		t.Fatalf("Failed to get absolute path to test directory: %v", err)
	}

	// Setup commands to create a PVC and a test pod
	setupCommands := []string{
		fmt.Sprintf("kubectl apply -f %s/pvc.yaml", testDir),
		fmt.Sprintf("kubectl apply -f %s/test-pod.yaml", testDir),
	}

	for _, cmd := range setupCommands {
		if err := runCommand(cmd); err != nil {
			t.Fatalf("Failed to execute setup command: %v", err)
		}
	}

	// Run the mount command
	err = Mount("default", "test-pvc", "/mnt/test", false)
	if err != nil {
		t.Fatalf("Failed to mount PVC: %v", err)
	}

	// Cleanup commands
	cleanupCommands := []string{
		fmt.Sprintf("kubectl delete -f %s/test-pod.yaml", testDir),
		fmt.Sprintf("kubectl delete -f %s/pvc.yaml", testDir),
	}

	for _, cmd := range cleanupCommands {
		if err := runCommand(cmd); err != nil {
			t.Fatalf("Failed to execute cleanup command: %v", err)
		}
	}
}

