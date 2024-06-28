package plugin

import (
	"fmt"
	"io/ioutil"
	"os/exec"
	"path/filepath"
	"testing"
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
	namespace := "default"
	pvcName := "test-pvc"
	localMountPoint := "/home/runner/work/pv-mounter/pv-mounter/foo"

	// Run the mount command using the tool
	mountCmd := fmt.Sprintf("/home/runner/work/pv-mounter/pv-mounter/pv-mounter mount %s %s %s", namespace, pvcName, localMountPoint)
	if err := runCommand(mountCmd); err != nil {
		t.Fatalf("Failed to mount PVC: %v", err)
	}

	// Verify that the filesystem is writable
	testFilePath := filepath.Join(localMountPoint, "testfile.txt")
	testData := []byte("This is a test file.")
	err := ioutil.WriteFile(testFilePath, testData, 0644)
	if err != nil {
		t.Fatalf("Failed to write to the mounted filesystem: %v", err)
	}

	// Verify that the file was written correctly
	readData, err := ioutil.ReadFile(testFilePath)
	if err != nil {
		t.Fatalf("Failed to read the test file from the mounted filesystem: %v", err)
	}
	if string(readData) != string(testData) {
		t.Fatalf("Data read from the test file does not match expected data: got %s, want %s", string(readData), string(testData))
	}

	// Run the unmount command using the tool
	unmountCmd := fmt.Sprintf("/home/runner/work/pv-mounter/pv-mounter/pv-mounter clean %s", localMountPoint)
	if err := runCommand(unmountCmd); err != nil {
		t.Fatalf("Failed to unmount the PVC: %v", err)
	}
}
