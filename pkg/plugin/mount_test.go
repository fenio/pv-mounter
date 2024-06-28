package plugin

import (
	"testing"
	"os/exec"
	"fmt"
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
	// Setup commands to create a PVC and a test pod
	setupCommands := []string{
		"kubectl apply -f test/pvc.yaml",
		"kubectl apply -f test/test-pod.yaml",
	}

	for _, cmd := range setupCommands {
		if err := runCommand(cmd); err != nil {
			t.Fatalf("Failed to execute setup command: %v", err)
		}
	}

	// Run the mount command
	err := Mount("default", "test-pvc", "/mnt/test", false)
	if err != nil {
		t.Fatalf("Failed to mount PVC: %v", err)
	}

	// Cleanup commands
	cleanupCommands := []string{
		"kubectl delete -f test/test-pod.yaml",
		"kubectl delete -f test/pvc.yaml",
	}

	for _, cmd := range cleanupCommands {
		if err := runCommand(cmd); err != nil {
			t.Fatalf("Failed to execute cleanup command: %v", err)
		}
	}
}



