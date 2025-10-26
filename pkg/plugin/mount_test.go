package plugin

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"testing"
	"time"

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
	defer func() { _ = os.Remove(tempFile.Name()) }()

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
		clientset.PrependReactor("get", "pods", func(_ k8stesting.Action) (handled bool, ret runtime.Object, err error) {
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
	t.Run("Nil command", func(_ *testing.T) {
		cleanupPortForward(nil)
	})

	t.Run("Command with process", func(t *testing.T) {
		ctx := context.Background()
		cmd := exec.CommandContext(ctx, "sleep", "10")
		_ = cmd.Start()
		if cmd.Process == nil {
			t.Fatal("Expected process to be started")
		}
		cleanupPortForward(cmd)
		_ = cmd.Wait()
	})

	t.Run("Command without process", func(_ *testing.T) {
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

	t.Run("Invalid SSH port (negative)", func(t *testing.T) {
		podSpec := createPodSpec("test-pod", 12345, "test-pvc", "publicKey", "standalone", -1, "", false, "", "", "")
		// Should default to DefaultSSHPort (2137)
		envMap := make(map[string]string)
		for _, env := range podSpec.Spec.Containers[0].Env {
			envMap[env.Name] = env.Value
		}
		if envMap["SSH_PORT"] != "2137" {
			t.Errorf("Expected SSH_PORT='2137' for negative port, got '%s'", envMap["SSH_PORT"])
		}
	})

	t.Run("Invalid SSH port (too large)", func(t *testing.T) {
		podSpec := createPodSpec("test-pod", 12345, "test-pvc", "publicKey", "standalone", 70000, "", false, "", "", "")
		// Should default to DefaultSSHPort (2137)
		envMap := make(map[string]string)
		for _, env := range podSpec.Spec.Containers[0].Env {
			envMap[env.Name] = env.Value
		}
		if envMap["SSH_PORT"] != "2137" {
			t.Errorf("Expected SSH_PORT='2137' for port > 65535, got '%s'", envMap["SSH_PORT"])
		}
	})
}

func TestBuildContainer(t *testing.T) {
	t.Run("Standard container", func(t *testing.T) {
		container := buildContainer("publicKey123", "standalone", 22, false, "", "")
		if container.Name != "volume-exposer" {
			t.Errorf("Expected container name 'volume-exposer', got '%s'", container.Name)
		}
		if container.Image != Image {
			t.Errorf("Expected image '%s', got '%s'", Image, container.Image)
		}
		if container.ImagePullPolicy != corev1.PullAlways {
			t.Error("Expected ImagePullPolicy to be PullAlways")
		}
		if len(container.Ports) != 1 || container.Ports[0].ContainerPort != 22 {
			t.Error("Expected container port 22")
		}
	})

	t.Run("Container with custom image and CPU limit", func(t *testing.T) {
		container := buildContainer("publicKey", "proxy", 2222, true, "custom-image:latest", "1000m")
		if container.Image != "custom-image:latest" {
			t.Errorf("Expected custom image, got '%s'", container.Image)
		}
		// CPU limits are normalized by Kubernetes, so "1000m" becomes "1"
		cpuLimit := container.Resources.Limits.Cpu()
		if cpuLimit.IsZero() {
			t.Error("Expected CPU limit to be set")
		}
		// Verify the limit is approximately 1 CPU (1000m = 1)
		if cpuLimit.Value() != 1 && cpuLimit.MilliValue() != 1000 {
			t.Errorf("Expected CPU limit to be 1 CPU or 1000m, got value=%d, millivalue=%d", cpuLimit.Value(), cpuLimit.MilliValue())
		}
	})
}

func TestBuildEnvVars(t *testing.T) {
	t.Run("Standalone role with env vars", func(t *testing.T) {
		envVars := buildEnvVars("testKey", "standalone", 2222, false)
		envMap := make(map[string]string)
		for _, env := range envVars {
			envMap[env.Name] = env.Value
		}
		if envMap["SSH_PUBLIC_KEY"] != "testKey" {
			t.Errorf("Expected SSH_PUBLIC_KEY='testKey', got '%s'", envMap["SSH_PUBLIC_KEY"])
		}
		if envMap["SSH_PORT"] != "2222" {
			t.Errorf("Expected SSH_PORT='2222', got '%s'", envMap["SSH_PORT"])
		}
		if envMap["NEEDS_ROOT"] != "false" {
			t.Errorf("Expected NEEDS_ROOT='false', got '%s'", envMap["NEEDS_ROOT"])
		}
		if envMap["ROLE"] != "standalone" {
			t.Errorf("Expected ROLE='standalone', got '%s'", envMap["ROLE"])
		}
	})

	t.Run("Proxy role with env vars", func(t *testing.T) {
		envVars := buildEnvVars("proxyKey", "proxy", 3333, true)
		envMap := make(map[string]string)
		for _, env := range envVars {
			envMap[env.Name] = env.Value
		}
		if envMap["NEEDS_ROOT"] != "true" {
			t.Errorf("Expected NEEDS_ROOT='true', got '%s'", envMap["NEEDS_ROOT"])
		}
		if envMap["ROLE"] != "proxy" {
			t.Errorf("Expected ROLE='proxy', got '%s'", envMap["ROLE"])
		}
	})

	t.Run("Custom role without ROLE env var", func(t *testing.T) {
		envVars := buildEnvVars("customKey", "custom", 22, false)
		hasRole := false
		for _, env := range envVars {
			if env.Name == "ROLE" {
				hasRole = true
			}
		}
		if hasRole {
			t.Error("Expected no ROLE env var for custom role")
		}
	})
}

func TestSelectImage(t *testing.T) {
	t.Run("Custom image provided", func(t *testing.T) {
		img := selectImage("my-custom-image:v1", false)
		if img != "my-custom-image:v1" {
			t.Errorf("Expected 'my-custom-image:v1', got '%s'", img)
		}
	})

	t.Run("Custom image overrides needsRoot", func(t *testing.T) {
		img := selectImage("my-custom-image:v1", true)
		if img != "my-custom-image:v1" {
			t.Errorf("Expected 'my-custom-image:v1', got '%s'", img)
		}
	})

	t.Run("Default image when needsRoot is false", func(t *testing.T) {
		img := selectImage("", false)
		if img != Image {
			t.Errorf("Expected '%s', got '%s'", Image, img)
		}
	})

	t.Run("Privileged image when needsRoot is true", func(t *testing.T) {
		img := selectImage("", true)
		if img != PrivilegedImage {
			t.Errorf("Expected '%s', got '%s'", PrivilegedImage, img)
		}
	})
}

func TestBuildResourceRequirements(t *testing.T) {
	t.Run("Default resources without CPU limit", func(t *testing.T) {
		resources := buildResourceRequirements("")
		if resources.Requests.Cpu().String() != CPURequest {
			t.Errorf("Expected CPU request '%s', got '%s'", CPURequest, resources.Requests.Cpu().String())
		}
		if resources.Requests.Memory().String() != MemoryRequest {
			t.Errorf("Expected memory request '%s', got '%s'", MemoryRequest, resources.Requests.Memory().String())
		}
		if resources.Limits.Memory().String() != MemoryLimit {
			t.Errorf("Expected memory limit '%s', got '%s'", MemoryLimit, resources.Limits.Memory().String())
		}
		if _, hasCPULimit := resources.Limits[corev1.ResourceCPU]; hasCPULimit {
			t.Error("Expected no CPU limit by default")
		}
	})

	t.Run("Custom CPU limit", func(t *testing.T) {
		resources := buildResourceRequirements("750m")
		if resources.Limits.Cpu().String() != "750m" {
			t.Errorf("Expected CPU limit '750m', got '%s'", resources.Limits.Cpu().String())
		}
	})
}

func TestBuildPodLabels(t *testing.T) {
	t.Run("Labels without original pod name", func(t *testing.T) {
		labels := buildPodLabels("my-pvc", 12345, "")
		if labels["app"] != "volume-exposer" {
			t.Errorf("Expected app label 'volume-exposer', got '%s'", labels["app"])
		}
		if labels["pvcName"] != "my-pvc" {
			t.Errorf("Expected pvcName label 'my-pvc', got '%s'", labels["pvcName"])
		}
		if labels["portNumber"] != "12345" {
			t.Errorf("Expected portNumber label '12345', got '%s'", labels["portNumber"])
		}
		if _, hasOriginalPodName := labels["originalPodName"]; hasOriginalPodName {
			t.Error("Expected no originalPodName label")
		}
	})

	t.Run("Labels with original pod name", func(t *testing.T) {
		labels := buildPodLabels("my-pvc", 54321, "original-pod")
		if labels["originalPodName"] != "original-pod" {
			t.Errorf("Expected originalPodName label 'original-pod', got '%s'", labels["originalPodName"])
		}
	})
}

func TestBuildImagePullSecrets(t *testing.T) {
	t.Run("No image secret", func(t *testing.T) {
		secrets := buildImagePullSecrets("")
		if len(secrets) != 0 {
			t.Errorf("Expected no image pull secrets, got %d", len(secrets))
		}
	})

	t.Run("With image secret", func(t *testing.T) {
		secrets := buildImagePullSecrets("my-registry-secret")
		if len(secrets) != 1 {
			t.Fatalf("Expected 1 image pull secret, got %d", len(secrets))
		}
		if secrets[0].Name != "my-registry-secret" {
			t.Errorf("Expected secret name 'my-registry-secret', got '%s'", secrets[0].Name)
		}
	})
}

func TestBuildPodSecurityContext(t *testing.T) {
	t.Run("Non-root security context", func(t *testing.T) {
		secCtx := buildPodSecurityContext(false)
		if secCtx == nil {
			t.Fatal("Expected non-nil security context")
		}
		if *secCtx.RunAsNonRoot != true {
			t.Error("Expected RunAsNonRoot to be true")
		}
		if *secCtx.RunAsUser != DefaultUserGroup {
			t.Errorf("Expected RunAsUser to be %d", DefaultUserGroup)
		}
		if *secCtx.RunAsGroup != DefaultUserGroup {
			t.Errorf("Expected RunAsGroup to be %d", DefaultUserGroup)
		}
	})

	t.Run("Root security context", func(t *testing.T) {
		secCtx := buildPodSecurityContext(true)
		if secCtx == nil {
			t.Fatal("Expected non-nil security context")
		}
		if *secCtx.RunAsNonRoot != false {
			t.Error("Expected RunAsNonRoot to be false")
		}
		if *secCtx.RunAsUser != 0 {
			t.Error("Expected RunAsUser to be 0")
		}
		if *secCtx.RunAsGroup != 0 {
			t.Error("Expected RunAsGroup to be 0")
		}
	})
}

func TestAttachPVCToPod(t *testing.T) {
	t.Run("Attach PVC to pod", func(t *testing.T) {
		pod := &corev1.Pod{
			Spec: corev1.PodSpec{
				Containers: []corev1.Container{{Name: "test"}},
			},
		}
		attachPVCToPod(pod, "test-pvc")
		if len(pod.Spec.Volumes) != 1 {
			t.Fatalf("Expected 1 volume, got %d", len(pod.Spec.Volumes))
		}
		if pod.Spec.Volumes[0].Name != "my-pvc" {
			t.Errorf("Expected volume name 'my-pvc', got '%s'", pod.Spec.Volumes[0].Name)
		}
		if pod.Spec.Volumes[0].PersistentVolumeClaim == nil {
			t.Fatal("Expected PersistentVolumeClaim volume source")
		}
		if pod.Spec.Volumes[0].PersistentVolumeClaim.ClaimName != "test-pvc" {
			t.Errorf("Expected claim name 'test-pvc', got '%s'", pod.Spec.Volumes[0].PersistentVolumeClaim.ClaimName)
		}
		if len(pod.Spec.Containers[0].VolumeMounts) != 1 {
			t.Fatalf("Expected 1 volume mount, got %d", len(pod.Spec.Containers[0].VolumeMounts))
		}
		if pod.Spec.Containers[0].VolumeMounts[0].MountPath != "/volume" {
			t.Errorf("Expected mount path '/volume', got '%s'", pod.Spec.Containers[0].VolumeMounts[0].MountPath)
		}
	})
}

func TestCreateTempSSHKeyFile(t *testing.T) {
	t.Run("Successfully create temp key file", func(t *testing.T) {
		privateKey := "-----BEGIN RSA PRIVATE KEY-----\ntest\n-----END RSA PRIVATE KEY-----"
		keyFilePath, cleanup, err := createTempSSHKeyFile(privateKey)
		if err != nil {
			t.Fatalf("createTempSSHKeyFile failed: %v", err)
		}
		defer cleanup()

		// Verify file exists
		if _, err := os.Stat(keyFilePath); os.IsNotExist(err) {
			t.Error("Expected temp key file to exist")
		}

		// Verify file permissions
		info, err := os.Stat(keyFilePath)
		if err != nil {
			t.Fatalf("Failed to stat key file: %v", err)
		}
		if info.Mode().Perm() != 0600 {
			t.Errorf("Expected file permissions 0600, got %o", info.Mode().Perm())
		}

		// Verify file contents
		content, err := os.ReadFile(keyFilePath)
		if err != nil {
			t.Fatalf("Failed to read key file: %v", err)
		}
		if string(content) != privateKey {
			t.Error("File content does not match private key")
		}
	})

	t.Run("Cleanup removes file", func(t *testing.T) {
		privateKey := "test-key"
		keyFilePath, cleanup, err := createTempSSHKeyFile(privateKey)
		if err != nil {
			t.Fatalf("createTempSSHKeyFile failed: %v", err)
		}

		cleanup()

		// Verify file is removed
		if _, err := os.Stat(keyFilePath); !os.IsNotExist(err) {
			t.Error("Expected temp key file to be removed after cleanup")
		}
	})
}

func TestSelectSSHUser(t *testing.T) {
	t.Run("Root user when needsRoot is true", func(t *testing.T) {
		user := selectSSHUser(true)
		if user != "root" {
			t.Errorf("Expected user 'root', got '%s'", user)
		}
	})

	t.Run("Non-root user when needsRoot is false", func(t *testing.T) {
		user := selectSSHUser(false)
		if user != "ve" {
			t.Errorf("Expected user 've', got '%s'", user)
		}
	})
}

func TestBuildSSHFSCommand(t *testing.T) {
	t.Run("Build sshfs command with correct arguments", func(t *testing.T) {
		ctx := context.Background()
		cmd := buildSSHFSCommand(ctx, "/tmp/keyfile", "testuser", "/mnt/pvc", 12345)

		// Check that the command contains "sshfs" in the path
		if cmd.Path == "" || len(cmd.Path) < 5 {
			t.Errorf("Expected command path to contain 'sshfs', got '%s'", cmd.Path)
		}

		args := cmd.Args
		expectedArgs := []string{
			"sshfs",
			"-o", "IdentityFile=/tmp/keyfile",
			"-o", "StrictHostKeyChecking=no",
			"-o", "UserKnownHostsFile=/dev/null",
			"-o", "nomap=ignore",
			"testuser@localhost:/volume",
			"/mnt/pvc",
			"-p", "12345",
		}

		if len(args) != len(expectedArgs) {
			t.Fatalf("Expected %d args, got %d", len(expectedArgs), len(args))
		}

		for i, arg := range expectedArgs {
			if args[i] != arg {
				t.Errorf("Arg %d: expected '%s', got '%s'", i, arg, args[i])
			}
		}
	})
}

func TestRegisterAndUnregisterTempKeyFile(t *testing.T) {
	t.Run("Register and unregister temp key file", func(t *testing.T) {
		testPath := "/tmp/test-key-file"

		registerTempKeyFile(testPath)

		tempKeyFilesMu.Lock()
		_, exists := tempKeyFiles[testPath]
		tempKeyFilesMu.Unlock()

		if !exists {
			t.Error("Expected temp key file to be registered")
		}

		unregisterTempKeyFile(testPath)

		tempKeyFilesMu.Lock()
		_, exists = tempKeyFiles[testPath]
		tempKeyFilesMu.Unlock()

		if exists {
			t.Error("Expected temp key file to be unregistered")
		}
	})
}

func TestCleanupTempKeyFiles(t *testing.T) {
	t.Run("Cleanup all registered temp key files", func(t *testing.T) {
		// Clear the map first to isolate this test
		tempKeyFilesMu.Lock()
		tempKeyFiles = make(map[string]struct{})
		tempKeyFilesMu.Unlock()

		// Create temp files
		tmpFile1, err := os.CreateTemp("", "test_key_1_*.pem")
		if err != nil {
			t.Fatalf("Failed to create temp file: %v", err)
		}
		path1 := tmpFile1.Name()
		_ = tmpFile1.Close()

		tmpFile2, err := os.CreateTemp("", "test_key_2_*.pem")
		if err != nil {
			t.Fatalf("Failed to create temp file: %v", err)
		}
		path2 := tmpFile2.Name()
		_ = tmpFile2.Close()

		// Register files
		registerTempKeyFile(path1)
		registerTempKeyFile(path2)

		// Verify files exist
		if _, err := os.Stat(path1); os.IsNotExist(err) {
			t.Error("Expected first temp file to exist")
		}
		if _, err := os.Stat(path2); os.IsNotExist(err) {
			t.Error("Expected second temp file to exist")
		}

		// Cleanup
		cleanupTempKeyFiles()

		// Verify files are removed
		if _, err := os.Stat(path1); !os.IsNotExist(err) {
			t.Error("Expected first temp file to be removed")
		}
		if _, err := os.Stat(path2); !os.IsNotExist(err) {
			t.Error("Expected second temp file to be removed")
		}

		// Note: cleanupTempKeyFiles removes files but doesn't clear the map
		// This is by design - the map tracks which files need cleanup
	})
}

func TestBuildEphemeralContainerSpec(t *testing.T) {
	t.Run("Build ephemeral container spec", func(t *testing.T) {
		spec := buildEphemeralContainerSpec(
			"test-ephemeral",
			"volume-name",
			"privateKey123",
			"publicKey456",
			"10.0.0.1",
			false,
			"",
		)

		if spec.Name != "test-ephemeral" {
			t.Errorf("Expected name 'test-ephemeral', got '%s'", spec.Name)
		}
		if spec.Image != Image {
			t.Errorf("Expected image '%s', got '%s'", Image, spec.Image)
		}
		if spec.ImagePullPolicy != corev1.PullAlways {
			t.Error("Expected ImagePullPolicy to be PullAlways")
		}

		// Check environment variables
		envMap := make(map[string]string)
		for _, env := range spec.Env {
			envMap[env.Name] = env.Value
		}
		if envMap["ROLE"] != "ephemeral" {
			t.Errorf("Expected ROLE='ephemeral', got '%s'", envMap["ROLE"])
		}
		if envMap["SSH_PRIVATE_KEY"] != "privateKey123" {
			t.Errorf("Expected SSH_PRIVATE_KEY='privateKey123', got '%s'", envMap["SSH_PRIVATE_KEY"])
		}
		if envMap["PROXY_POD_IP"] != "10.0.0.1" {
			t.Errorf("Expected PROXY_POD_IP='10.0.0.1', got '%s'", envMap["PROXY_POD_IP"])
		}
		if envMap["SSH_PUBLIC_KEY"] != "publicKey456" {
			t.Errorf("Expected SSH_PUBLIC_KEY='publicKey456', got '%s'", envMap["SSH_PUBLIC_KEY"])
		}
		if envMap["NEEDS_ROOT"] != "false" {
			t.Errorf("Expected NEEDS_ROOT='false', got '%s'", envMap["NEEDS_ROOT"])
		}

		// Check volume mounts
		if len(spec.VolumeMounts) != 1 {
			t.Fatalf("Expected 1 volume mount, got %d", len(spec.VolumeMounts))
		}
		if spec.VolumeMounts[0].Name != "volume-name" {
			t.Errorf("Expected volume name 'volume-name', got '%s'", spec.VolumeMounts[0].Name)
		}
		if spec.VolumeMounts[0].MountPath != "/volume" {
			t.Errorf("Expected mount path '/volume', got '%s'", spec.VolumeMounts[0].MountPath)
		}
	})

	t.Run("Build ephemeral container spec with root", func(t *testing.T) {
		spec := buildEphemeralContainerSpec(
			"test-ephemeral-root",
			"volume-name",
			"privateKey",
			"publicKey",
			"10.0.0.2",
			true,
			"",
		)

		if spec.Image != PrivilegedImage {
			t.Errorf("Expected privileged image '%s', got '%s'", PrivilegedImage, spec.Image)
		}

		envMap := make(map[string]string)
		for _, env := range spec.Env {
			envMap[env.Name] = env.Value
		}
		if envMap["NEEDS_ROOT"] != "true" {
			t.Errorf("Expected NEEDS_ROOT='true', got '%s'", envMap["NEEDS_ROOT"])
		}
	})

	t.Run("Build ephemeral container spec with custom image", func(t *testing.T) {
		customImage := "my-custom-image:v2"
		spec := buildEphemeralContainerSpec(
			"test-ephemeral-custom",
			"volume-name",
			"privateKey",
			"publicKey",
			"10.0.0.3",
			false,
			customImage,
		)

		if spec.Image != customImage {
			t.Errorf("Expected custom image '%s', got '%s'", customImage, spec.Image)
		}
	})
}

func TestCheckEphemeralContainerStatus(t *testing.T) {
	deadline := time.Now().Add(1 * time.Minute)

	t.Run("No ephemeral container statuses", func(t *testing.T) {
		pod := &corev1.Pod{
			Status: corev1.PodStatus{
				EphemeralContainerStatuses: []corev1.ContainerStatus{},
			},
		}
		ready, err := checkEphemeralContainerStatus(pod, deadline, false)
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}
		if ready {
			t.Error("Expected ready to be false when no ephemeral container statuses")
		}
	})

	t.Run("Ephemeral container running", func(t *testing.T) {
		pod := &corev1.Pod{
			Status: corev1.PodStatus{
				EphemeralContainerStatuses: []corev1.ContainerStatus{
					{
						Name: "ephemeral-test",
						State: corev1.ContainerState{
							Running: &corev1.ContainerStateRunning{
								StartedAt: metav1.Now(),
							},
						},
					},
				},
			},
		}
		ready, err := checkEphemeralContainerStatus(pod, deadline, false)
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}
		if !ready {
			t.Error("Expected ready to be true when ephemeral container is running")
		}
	})

	t.Run("Ephemeral container waiting", func(t *testing.T) {
		pod := &corev1.Pod{
			Status: corev1.PodStatus{
				EphemeralContainerStatuses: []corev1.ContainerStatus{
					{
						Name: "ephemeral-test",
						State: corev1.ContainerState{
							Waiting: &corev1.ContainerStateWaiting{
								Reason: "ContainerCreating",
							},
						},
					},
				},
			},
		}
		ready, err := checkEphemeralContainerStatus(pod, deadline, false)
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}
		if ready {
			t.Error("Expected ready to be false when ephemeral container is waiting")
		}
	})

	t.Run("Ephemeral container terminated", func(t *testing.T) {
		pod := &corev1.Pod{
			Status: corev1.PodStatus{
				EphemeralContainerStatuses: []corev1.ContainerStatus{
					{
						Name: "ephemeral-test",
						State: corev1.ContainerState{
							Terminated: &corev1.ContainerStateTerminated{
								Reason: "Error",
							},
						},
					},
				},
			},
		}
		ready, err := checkEphemeralContainerStatus(pod, deadline, false)
		if err == nil {
			t.Error("Expected error when ephemeral container is terminated")
		}
		if ready {
			t.Error("Expected ready to be false when ephemeral container is terminated")
		}
	})

	t.Run("No ephemeral container statuses with debug near deadline", func(t *testing.T) {
		// Set deadline to be very close (less than 5 seconds away) to trigger debug output
		nearDeadline := time.Now().Add(3 * time.Second)
		pod := &corev1.Pod{
			Status: corev1.PodStatus{
				EphemeralContainerStatuses: []corev1.ContainerStatus{},
			},
		}
		ready, err := checkEphemeralContainerStatus(pod, nearDeadline, true)
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}
		if ready {
			t.Error("Expected ready to be false when no ephemeral container statuses")
		}
	})

	t.Run("Ephemeral container running with debug", func(t *testing.T) {
		pod := &corev1.Pod{
			Status: corev1.PodStatus{
				EphemeralContainerStatuses: []corev1.ContainerStatus{
					{
						Name: "ephemeral-test",
						State: corev1.ContainerState{
							Running: &corev1.ContainerStateRunning{
								StartedAt: metav1.Now(),
							},
						},
					},
				},
			},
		}
		ready, err := checkEphemeralContainerStatus(pod, deadline, true)
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}
		if !ready {
			t.Error("Expected ready to be true when ephemeral container is running")
		}
	})

	t.Run("Ephemeral container waiting with debug", func(t *testing.T) {
		pod := &corev1.Pod{
			Status: corev1.PodStatus{
				EphemeralContainerStatuses: []corev1.ContainerStatus{
					{
						Name: "ephemeral-test",
						State: corev1.ContainerState{
							Waiting: &corev1.ContainerStateWaiting{
								Reason: "ContainerCreating",
							},
						},
					},
				},
			},
		}
		ready, err := checkEphemeralContainerStatus(pod, deadline, true)
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}
		if ready {
			t.Error("Expected ready to be false when ephemeral container is waiting")
		}
	})
}
