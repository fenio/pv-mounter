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
	podSpec := createPodSpec("test-pod", 12345, "test-pvc", "publicKey", "standalone", 22, "", false, "whatever", "secret", "300m")
	if podSpec.Name != "test-pod" {
		t.Errorf("Expected pod name 'test-pod', got '%s'", podSpec.Name)
	}
	// Additional checks for volumes, containers, etc.
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
}
