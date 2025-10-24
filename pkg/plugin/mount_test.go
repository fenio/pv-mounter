package plugin

import (
	"context"
	"fmt"
	"os"
	"os/exec"
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

func TestGetPVCVolumeName_NoPVC(t *testing.T) {
	pod := &corev1.Pod{
		Spec: corev1.PodSpec{
			Volumes: []corev1.Volume{
				{
					Name: "test-volume",
					VolumeSource: corev1.VolumeSource{
						EmptyDir: &corev1.EmptyDirVolumeSource{},
					},
				},
			},
		},
	}
	_, err := getPVCVolumeName(pod)
	if err == nil {
		t.Error("Expected error when no PVC volume exists")
	}
}

func TestCheckPVCUsage(t *testing.T) {
	namespace := "default"
	pvcName := "test-pvc"

	t.Run("PVC exists and is bound", func(t *testing.T) {
		clientset := fake.NewSimpleClientset(&corev1.PersistentVolumeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Name:      pvcName,
				Namespace: namespace,
			},
			Status: corev1.PersistentVolumeClaimStatus{
				Phase: corev1.ClaimBound,
			},
		})

		ctx := context.Background()
		pvc, err := checkPVCUsage(ctx, clientset, namespace, pvcName)
		if err != nil {
			t.Errorf("checkPVCUsage returned an error: %v", err)
		}
		if pvc.Name != pvcName {
			t.Errorf("Expected PVC name '%s', got '%s'", pvcName, pvc.Name)
		}
	})

	t.Run("PVC not bound", func(t *testing.T) {
		clientset := fake.NewSimpleClientset(&corev1.PersistentVolumeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Name:      pvcName,
				Namespace: namespace,
			},
			Status: corev1.PersistentVolumeClaimStatus{
				Phase: corev1.ClaimPending,
			},
		})

		ctx := context.Background()
		_, err := checkPVCUsage(ctx, clientset, namespace, pvcName)
		if err == nil {
			t.Error("Expected error when PVC is not bound")
		}
	})

	t.Run("PVC does not exist", func(t *testing.T) {
		clientset := fake.NewSimpleClientset()

		ctx := context.Background()
		_, err := checkPVCUsage(ctx, clientset, namespace, pvcName)
		if err == nil {
			t.Error("Expected error when PVC does not exist")
		}
	})
}

func TestCleanupPortForward(t *testing.T) {
	t.Run("Nil command", func(t *testing.T) {
		cleanupPortForward(nil)
	})

	t.Run("Command with process", func(t *testing.T) {
		cmd := exec.Command("sleep", "10")
		cmd.Start()
		if cmd.Process == nil {
			t.Fatal("Expected process to be started")
		}
		cleanupPortForward(cmd)
		cmd.Wait()
	})

	t.Run("Command without process", func(t *testing.T) {
		cmd := &exec.Cmd{}
		cleanupPortForward(cmd)
	})
}

func TestGetSecurityContext(t *testing.T) {
	t.Run("Non-root security context", func(t *testing.T) {
		secCtx := getSecurityContext(false)
		if secCtx == nil {
			t.Fatal("Expected non-nil security context")
		}
		if *secCtx.AllowPrivilegeEscalation != false {
			t.Error("Expected AllowPrivilegeEscalation to be false")
		}
		if *secCtx.ReadOnlyRootFilesystem != true {
			t.Error("Expected ReadOnlyRootFilesystem to be true")
		}
		if *secCtx.RunAsNonRoot != true {
			t.Error("Expected RunAsNonRoot to be true")
		}
		if *secCtx.RunAsUser != DefaultID {
			t.Errorf("Expected RunAsUser to be %d", DefaultID)
		}
		if *secCtx.RunAsGroup != DefaultID {
			t.Errorf("Expected RunAsGroup to be %d", DefaultID)
		}
		if secCtx.Capabilities == nil || len(secCtx.Capabilities.Drop) == 0 {
			t.Error("Expected capabilities to be dropped")
		}
		if secCtx.Capabilities.Drop[0] != "ALL" {
			t.Errorf("Expected ALL capability to be dropped, got %v", secCtx.Capabilities.Drop[0])
		}
		if secCtx.SeccompProfile == nil || secCtx.SeccompProfile.Type != corev1.SeccompProfileTypeRuntimeDefault {
			t.Error("Expected SeccompProfile to be RuntimeDefault")
		}
	})

	t.Run("Root security context", func(t *testing.T) {
		secCtx := getSecurityContext(true)
		if secCtx == nil {
			t.Fatal("Expected non-nil security context")
		}
		if *secCtx.AllowPrivilegeEscalation != true {
			t.Error("Expected AllowPrivilegeEscalation to be true for root")
		}
		if *secCtx.ReadOnlyRootFilesystem != true {
			t.Error("Expected ReadOnlyRootFilesystem to be true")
		}
		if secCtx.RunAsNonRoot != nil {
			t.Error("Expected RunAsNonRoot to be nil for root")
		}
		if secCtx.RunAsUser != nil {
			t.Error("Expected RunAsUser to be nil for root")
		}
		if secCtx.RunAsGroup != nil {
			t.Error("Expected RunAsGroup to be nil for root")
		}
		if secCtx.Capabilities == nil || len(secCtx.Capabilities.Add) == 0 {
			t.Error("Expected capabilities to be added for root")
		}
		hasAdmin := false
		hasChroot := false
		for _, cap := range secCtx.Capabilities.Add {
			if cap == "SYS_ADMIN" {
				hasAdmin = true
			}
			if cap == "SYS_CHROOT" {
				hasChroot = true
			}
		}
		if !hasAdmin || !hasChroot {
			t.Error("Expected SYS_ADMIN and SYS_CHROOT capabilities for root")
		}
		if secCtx.SeccompProfile == nil || secCtx.SeccompProfile.Type != corev1.SeccompProfileTypeRuntimeDefault {
			t.Error("Expected SeccompProfile to be RuntimeDefault")
		}
	})
}

func TestGeneratePodNameAndPort_Roles(t *testing.T) {
	t.Run("Proxy role", func(t *testing.T) {
		name, port := generatePodNameAndPort("proxy")
		if len(name) < len("volume-exposer-proxy-") {
			t.Errorf("Expected proxy pod name to start with 'volume-exposer-proxy-', got '%s'", name)
		}
		if port < 1024 || port > 65535 {
			t.Errorf("Expected port in valid range (1024-65535), got %d", port)
		}
	})

	t.Run("Standalone role", func(t *testing.T) {
		name, port := generatePodNameAndPort("standalone")
		if len(name) < len("volume-exposer-") {
			t.Errorf("Expected standalone pod name to start with 'volume-exposer-', got '%s'", name)
		}
		if port < 1024 || port > 65535 {
			t.Errorf("Expected port in valid range (1024-65535), got %d", port)
		}
	})
}

func TestCreatePodSpec_Comprehensive(t *testing.T) {
	t.Run("With image secret", func(t *testing.T) {
		podSpec := createPodSpec("test-pod", 12345, "test-pvc", "publicKey", "standalone", 22, "", false, "", "my-secret", "")
		if len(podSpec.Spec.ImagePullSecrets) == 0 {
			t.Error("Expected image pull secrets to be set")
		}
		if podSpec.Spec.ImagePullSecrets[0].Name != "my-secret" {
			t.Errorf("Expected image pull secret 'my-secret', got '%s'", podSpec.Spec.ImagePullSecrets[0].Name)
		}
	})

	t.Run("Proxy role with original pod name", func(t *testing.T) {
		podSpec := createPodSpec("test-pod", 12345, "test-pvc", "publicKey", "proxy", 6666, "original-pod", false, "", "", "")
		if podSpec.Name != "test-pod" {
			t.Errorf("Expected pod name 'test-pod', got '%s'", podSpec.Name)
		}
		if len(podSpec.Spec.Volumes) != 0 {
			t.Error("Expected no volumes for proxy role")
		}
		if podSpec.Labels["originalPodName"] != "original-pod" {
			t.Errorf("Expected originalPodName label 'original-pod', got '%s'", podSpec.Labels["originalPodName"])
		}
	})

	t.Run("Root mode", func(t *testing.T) {
		podSpec := createPodSpec("test-pod", 12345, "test-pvc", "publicKey", "standalone", 22, "", true, "", "", "")
		if *podSpec.Spec.SecurityContext.RunAsNonRoot != false {
			t.Error("Expected RunAsNonRoot to be false in root mode")
		}
		if *podSpec.Spec.SecurityContext.RunAsUser != 0 {
			t.Error("Expected RunAsUser to be 0 in root mode")
		}
		if *podSpec.Spec.SecurityContext.RunAsGroup != 0 {
			t.Error("Expected RunAsGroup to be 0 in root mode")
		}
	})

	t.Run("Custom image", func(t *testing.T) {
		customImage := "myregistry/custom-image:v1.0"
		podSpec := createPodSpec("test-pod", 12345, "test-pvc", "publicKey", "standalone", 22, "", false, customImage, "", "")
		if podSpec.Spec.Containers[0].Image != customImage {
			t.Errorf("Expected custom image '%s', got '%s'", customImage, podSpec.Spec.Containers[0].Image)
		}
	})

	t.Run("CPU limit", func(t *testing.T) {
		cpuLimit := "500m"
		podSpec := createPodSpec("test-pod", 12345, "test-pvc", "publicKey", "standalone", 22, "", false, "", "", cpuLimit)
		if podSpec.Spec.Containers[0].Resources.Limits.Cpu().String() != cpuLimit {
			t.Errorf("Expected CPU limit '%s', got '%s'", cpuLimit, podSpec.Spec.Containers[0].Resources.Limits.Cpu().String())
		}
	})

	t.Run("Default non-root image", func(t *testing.T) {
		podSpec := createPodSpec("test-pod", 12345, "test-pvc", "publicKey", "standalone", 22, "", false, "", "", "")
		if podSpec.Spec.Containers[0].Image != Image {
			t.Errorf("Expected default image '%s', got '%s'", Image, podSpec.Spec.Containers[0].Image)
		}
	})

	t.Run("Default privileged image", func(t *testing.T) {
		podSpec := createPodSpec("test-pod", 12345, "test-pvc", "publicKey", "standalone", 22, "", true, "", "", "")
		if podSpec.Spec.Containers[0].Image != PrivilegedImage {
			t.Errorf("Expected privileged image '%s', got '%s'", PrivilegedImage, podSpec.Spec.Containers[0].Image)
		}
	})

	t.Run("Environment variables", func(t *testing.T) {
		podSpec := createPodSpec("test-pod", 12345, "test-pvc", "testPublicKey", "standalone", 2137, "", false, "", "", "")
		envMap := make(map[string]string)
		for _, env := range podSpec.Spec.Containers[0].Env {
			envMap[env.Name] = env.Value
		}
		if envMap["SSH_PUBLIC_KEY"] != "testPublicKey" {
			t.Errorf("Expected SSH_PUBLIC_KEY='testPublicKey', got '%s'", envMap["SSH_PUBLIC_KEY"])
		}
		if envMap["SSH_PORT"] != "2137" {
			t.Errorf("Expected SSH_PORT='2137', got '%s'", envMap["SSH_PORT"])
		}
		if envMap["NEEDS_ROOT"] != "false" {
			t.Errorf("Expected NEEDS_ROOT='false', got '%s'", envMap["NEEDS_ROOT"])
		}
		if envMap["ROLE"] != "standalone" {
			t.Errorf("Expected ROLE='standalone', got '%s'", envMap["ROLE"])
		}
	})

	t.Run("Without ROLE env var for non-standard role", func(t *testing.T) {
		podSpec := createPodSpec("test-pod", 12345, "test-pvc", "testPublicKey", "custom", 2137, "", false, "", "", "")
		hasRole := false
		for _, env := range podSpec.Spec.Containers[0].Env {
			if env.Name == "ROLE" {
				hasRole = true
			}
		}
		if hasRole {
			t.Error("Expected no ROLE env var for non-standard role")
		}
	})
}
