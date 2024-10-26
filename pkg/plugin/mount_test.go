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

func TestCheckPVAccessMode(t *testing.T) {
	// Create a fake clientset
	clientset := fake.NewSimpleClientset()

	// Create a test PV and PVC
	pv := &corev1.PersistentVolume{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-pv",
		},
		Spec: corev1.PersistentVolumeSpec{
			AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
		},
	}
	pvc := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pvc",
			Namespace: "default",
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			VolumeName: "test-pv",
		},
	}
	_, err := clientset.CoreV1().PersistentVolumes().Create(context.TODO(), pv, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("Error creating test PV: %v", err)
	}
	_, err = clientset.CoreV1().PersistentVolumeClaims("default").Create(context.TODO(), pvc, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("Error creating test PVC: %v", err)
	}

	// Call the checkPVAccessMode function
	canBeMounted, podUsingPVC, err := checkPVAccessMode(context.TODO(), clientset, pvc, "default")
	if err != nil {
		t.Errorf("checkPVAccessMode function returned an error: %v", err)
	}
	if !canBeMounted {
		t.Error("checkPVAccessMode function returned false for canBeMounted, expected true")
	}
	if podUsingPVC != "" {
		t.Errorf("checkPVAccessMode function returned non-empty podUsingPVC: %s", podUsingPVC)
	}
}
