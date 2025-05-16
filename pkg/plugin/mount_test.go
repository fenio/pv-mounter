package plugin

import (
	"context"
	"fmt"
	"os"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"
)

func TestValidateMountPoint(t *testing.T) {
	t.Run("Mount point exists", func(t *testing.T) {
		// Create a temporary directory to simulate an existing mount point
		tempDir := t.TempDir()

		// Call the function
		err := validateMountPoint(tempDir)

		// Check the result
		if err != nil {
			t.Errorf("validateMountPoint(%s) returned an unexpected error: %v", tempDir, err)
		}
	})

	t.Run("Mount point does not exist", func(t *testing.T) {
		// Define a path that does not exist
		nonExistentPath := "/path/that/does/not/exist"

		// Call the function
		err := validateMountPoint(nonExistentPath)

		// Check the result
		if err == nil {
			t.Errorf("validateMountPoint(%s) should have returned an error, but it did not", nonExistentPath)
		}
	})
}

func TestValidateMountPoint_FileInsteadOfDirectory(t *testing.T) {
	// Create a temporary file to simulate a file instead of a directory
	tempFile, err := os.CreateTemp("", "testfile")
	if err != nil {
		t.Fatalf("Failed to create temporary file: %v", err)
	}
	defer os.Remove(tempFile.Name())

	// Call the function
	err = validateMountPoint(tempFile.Name())

	// Check the result
	if err != nil {
		t.Errorf("validateMountPoint(%s) returned an unexpected error: %v", tempFile.Name(), err)
	}
}

func TestGetPodIP(t *testing.T) {
	namespace := "default"
	podName := "test-pod"
	podIP := "192.168.1.1"

	t.Run("Pod exists", func(t *testing.T) {
		// Create a fake clientset
		clientset := fake.NewSimpleClientset(&corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      podName,
				Namespace: namespace,
			},
			Status: corev1.PodStatus{
				PodIP: podIP,
			},
		})

		// Create a context
		ctx := context.Background()

		// Call the function
		ip, err := getPodIP(ctx, clientset, namespace, podName)
		if err != nil {
			t.Errorf("getPodIP() returned an error: %v", err)
		}
		if ip != podIP {
			t.Errorf("getPodIP() returned IP %s; want %s", ip, podIP)
		}
	})

	t.Run("Pod does not exist", func(t *testing.T) {
		// Create a fake clientset with no pods
		clientset := fake.NewSimpleClientset()

		// Create a context
		ctx := context.Background()

		// Call the function
		ip, err := getPodIP(ctx, clientset, namespace, podName)
		if err == nil {
			t.Errorf("getPodIP() did not return an error for non-existent pod")
		}
		if ip != "" {
			t.Errorf("getPodIP() returned IP %s; want empty string", ip)
		}
	})

	t.Run("API error", func(t *testing.T) {
		// Create a fake clientset that will return an error
		clientset := fake.NewSimpleClientset()
		clientset.PrependReactor("get", "pods", func(action k8stesting.Action) (handled bool, ret runtime.Object, err error) {
			return true, nil, fmt.Errorf("API error")
		})

		// Create a context
		ctx := context.Background()

		// Call the function
		ip, err := getPodIP(ctx, clientset, namespace, podName)
		if err == nil {
			t.Errorf("getPodIP() did not return an error for API error")
		}
		if ip != "" {
			t.Errorf("getPodIP() returned IP %s; want empty string", ip)
		}
	})
}

func TestContains(t *testing.T) {
	modes := []corev1.PersistentVolumeAccessMode{
		corev1.ReadWriteOnce,
		corev1.ReadWriteMany,
	}
	if !contains(modes, corev1.ReadWriteOnce) {
		t.Error("Expected mode to be found")
	}
	if contains(modes, corev1.ReadOnlyMany) {
		t.Error("Did not expect mode to be found")
	}
}

func TestGeneratePodNameAndPort(t *testing.T) {
	name1, port1 := generatePodNameAndPort("standalone")
	name2, port2 := generatePodNameAndPort("standalone")
	if name1 == name2 {
		t.Error("Expected different pod names")
	}
	if port1 == port2 {
		t.Error("Expected different ports")
	}
}

func TestCreatePodSpec(t *testing.T) {
	podSpec := createPodSpec("test-pod", 12345, "test-pvc", "publicKey", "standalone", 22, "", false, "whatever", "secret", "")
	if podSpec.Name != "test-pod" {
		t.Errorf("Expected pod name 'test-pod', got '%s'", podSpec.Name)
	}
	// Additional checks for volumes, containers, etc.

	// Test cpuLimit propagation
	cpuLimit := "250m"
	podSpecWithLimit := createPodSpec("test-pod2", 23456, "test-pvc2", "publicKey2", "standalone", 22, "", false, "whatever", "secret", cpuLimit)
	limit := podSpecWithLimit.Spec.Containers[0].Resources.Limits
	if limit == nil || limit.Cpu().String() != cpuLimit {
		t.Errorf("Expected CPU limit '%s', got '%s'", cpuLimit, limit.Cpu().String())
	}
}

func TestGetPVCVolumeName(t *testing.T) {
	pod := &corev1.Pod{
		Spec: corev1.PodSpec{
			Volumes: []corev1.Volume{
				{
					Name: "test-volume",
					VolumeSource: corev1.VolumeSource{
						PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
							ClaimName: "test-pvc",
						},
					},
				},
			},
		},
	}
	volumeName, err := getPVCVolumeName(pod)
	if err != nil {
		t.Errorf("getPVCVolumeName returned an error: %v", err)
	}
	if volumeName != "test-volume" {
		t.Errorf("Expected volume name 'test-volume', got '%s'", volumeName)
	}

	// Test error branch
	podEmpty := &corev1.Pod{}
	_, err = getPVCVolumeName(podEmpty)
	if err == nil {
		t.Error("Expected error for pod with no PVC volumes")
	}
}

// --- Additional tests for improved coverage ---

func TestCheckPVCUsage_Errors(t *testing.T) {
	clientset := fake.NewSimpleClientset()
	ctx := context.Background()

	// PVC does not exist
	_, err := checkPVCUsage(ctx, clientset, "default", "missing-pvc")
	if err == nil {
		t.Error("Expected error for missing PVC")
	}

	// PVC not bound
	pvc := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{Name: "unbound-pvc", Namespace: "default"},
		Status:     corev1.PersistentVolumeClaimStatus{Phase: corev1.ClaimPending},
	}
	clientset = fake.NewSimpleClientset(pvc)
	_, err = checkPVCUsage(ctx, clientset, "default", "unbound-pvc")
	if err == nil || !strings.Contains(err.Error(), "not bound") {
		t.Error("Expected not bound error")
	}
}

func TestSetupPod_CreateError(t *testing.T) {
	clientset := fake.NewSimpleClientset()
	ctx := context.Background()
	// Simulate error by creating a pod with the same name first
	pod := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "volume-exposer-abcde", Namespace: "default"}}
	clientset.CoreV1().Pods("default").Create(ctx, pod, metav1.CreateOptions{})

	// Use a fixed pod name by patching generatePodNameAndPort for this test
	origGen := generatePodNameAndPort
	generatePodNameAndPort = func(role string) (string, int) {
		return "volume-exposer-abcde", 12345
	}
	defer func() { generatePodNameAndPort = origGen }()

	_, _, err := setupPod(ctx, clientset, "default", "pvc", "pubkey", "standalone", 2137, "", false, "", "", "")
	if err == nil {
		t.Error("Expected error when pod already exists")
	}
}

func TestValidateMountPoint_FileInsteadOfDirectory_Error(t *testing.T) {
	// Remove file after test
	tempFile, err := os.CreateTemp("", "testfile")
	if err != nil {
		t.Fatalf("Failed to create temporary file: %v", err)
	}
	defer os.Remove(tempFile.Name())

	// Should not error for file (current implementation), but let's check
	err = validateMountPoint(tempFile.Name())
	if err != nil {
		t.Errorf("validateMountPoint(%s) returned an unexpected error: %v", tempFile.Name(), err)
	}
}

func TestWaitForPodReady_Error(t *testing.T) {
	clientset := fake.NewSimpleClientset()
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // immediately cancel context to force error

	err := waitForPodReady(ctx, clientset, "default", "nonexistent-pod")
	if err == nil {
		t.Error("Expected error due to cancelled context")
	}
}

func TestCheckPVAccessMode_Error(t *testing.T) {
	clientset := fake.NewSimpleClientset()
	ctx := context.Background()
	pvc := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{Name: "pvc", Namespace: "default"},
		Spec: corev1.PersistentVolumeClaimSpec{
			VolumeName: "pv1",
		},
	}
	// No PV exists, should error
	_, _, err := checkPVAccessMode(ctx, clientset, pvc, "default")
	if err == nil {
		t.Error("Expected error when PV does not exist")
	}
}

func TestHandleRWX_ErrorBranches(t *testing.T) {
	// Simulate error in GenerateKeyPair
	origGen := GenerateKeyPair
	GenerateKeyPair = func(curve elliptic.Curve) (string, string, error) {
		return "", "", fmt.Errorf("fail")
	}
	defer func() { GenerateKeyPair = origGen }()

	clientset := fake.NewSimpleClientset()
	ctx := context.Background()
	err := handleRWX(ctx, clientset, "default", "pvc", "/tmp", false, false, "", "", "")
	if err == nil || !strings.Contains(err.Error(), "error generating key pair") {
		t.Error("Expected error from GenerateKeyPair")
	}
}

func TestHandleRWO_ErrorBranches(t *testing.T) {
	// Simulate error in GenerateKeyPair
	origGen := GenerateKeyPair
	GenerateKeyPair = func(curve elliptic.Curve) (string, string, error) {
		return "", "", fmt.Errorf("fail")
	}
	defer func() { GenerateKeyPair = origGen }()

	clientset := fake.NewSimpleClientset()
	ctx := context.Background()
	err := handleRWO(ctx, clientset, "default", "pvc", "/tmp", "pod", false, false, "", "", "")
	if err == nil || !strings.Contains(err.Error(), "error generating key pair") {
		t.Error("Expected error from GenerateKeyPair")
	}
}

func TestMountPVCOverSSH_Error(t *testing.T) {
	// Save and restore execCommand
	origLookPath := exec.LookPath
	origCommand := exec.Command
	defer func() {
		exec.LookPath = origLookPath
		exec.Command = origCommand
	}()

	// Simulate sshfs not found
	exec.LookPath = func(file string) (string, error) {
		return "", fmt.Errorf("not found")
	}
	err := mountPVCOverSSH(2222, "/tmp", "pvc", "key", false)
	if err == nil {
		t.Error("Expected error when sshfs is not found")
	}

	// Simulate sshfs command fails
	exec.LookPath = origLookPath
	exec.Command = func(name string, arg ...string) *exec.Cmd {
		return exec.Command("false") // always fails
	}
	err = mountPVCOverSSH(2222, "/tmp", "pvc", "key", false)
	if err == nil {
		t.Error("Expected error when sshfs command fails")
	}
}

func TestCreateEphemeralContainer_ErrorBranches(t *testing.T) {
	clientset := fake.NewSimpleClientset()
	ctx := context.Background()
	// Pod does not exist
	err := createEphemeralContainer(ctx, clientset, "default", "missing-pod", "priv", "pub", "ip", false, "")
	if err == nil {
		t.Error("Expected error when pod does not exist")
	}

	// Pod exists but no PVC volume
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "pod", Namespace: "default"},
	}
	clientset = fake.NewSimpleClientset(pod)
	err = createEphemeralContainer(ctx, clientset, "default", "pod", "priv", "pub", "ip", false, "")
	if err == nil {
		t.Error("Expected error when no PVC volume")
	}
}
