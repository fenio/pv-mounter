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
	// Run the mount command
	err := Mount("default", "test-pvc", "/mnt/test", false)
	if err != nil {
		t.Fatalf("Failed to mount PVC: %v", err)
	}
}

