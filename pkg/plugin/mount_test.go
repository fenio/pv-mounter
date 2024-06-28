package plugin

import (
	"testing"
	"os/exec"
	"fmt"
	"path/filepath"
	"os"
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
	// Get the current working directory
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get current working directory: %v", err)
	}

	// Construct the absolute paths to the test files
	pvcPath := filepath.Join(cwd, "pkg/plugin/test/pvc.yaml")
	podPath := filepath.Join(cwd, "pkg/plugin/test/test-pod.yaml")

	// Setup commands to create a PVC and a test pod
	setupCommands := []string{
		fmt.Sprintf("kubectl apply -f %s", pvcPath),
		fmt.Sprintf("kubectl apply -f %s", podPath),
	}

	for _, cmd := range setupCommands {
		if err := runCommand(cmd); err != nil {
			t.Fatalf("Failed to execute setup command: %v", err)
		}
	}

	// Wait for the PVC to be bound
	checkPVCBound := fmt.Sprintf("kubectl wait --for=condition=Bound pvc/test-pvc --timeout=2m")
	if err := runCommand(checkPVCBound); err != nil {
		t.Fatalf("PVC test-pvc is not bound: %v", err)
	}

	// Run the mount command
	err = Mount("default", "test-pvc", "/mnt/test", false)
	if err != nil {
		t.Fatalf("Failed to mount PVC: %v", err)
	}

	// Cleanup commands
	cleanupCommands := []string{
		fmt.Sprintf("kubectl delete -f %s", podPath),
		fmt.Sprintf("kubectl delete -f %s", pvcPath),
	}

	for _, cmd := range cleanupCommands {
		if err := runCommand(cmd); err != nil {
			t.Fatalf("Failed to execute cleanup command: %v", err)
		}
	}
}

