package plugin

import (
	"context"
	"os"
	"os/exec"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestValidateMountPoint(t *testing.T) {
	t.Run("Mount point exists", func(t *testing.T) {
		tempDir := t.TempDir()

		err := validateMountPoint(tempDir)

		if err != nil {
			t.Errorf("validateMountPoint(%s) returned an unexpected error: %v", tempDir, err)
		}
	})

	t.Run("Mount point does not exist", func(t *testing.T) {
		nonExistentPath := "/path/that/does/not/exist"

		err := validateMountPoint(nonExistentPath)

		if err == nil {
			t.Errorf("validateMountPoint(%s) should have returned an error, but it did not", nonExistentPath)
		}
	})
}

func TestValidateMountPoint_FileInsteadOfDirectory(t *testing.T) {
	tempFile, err := os.CreateTemp("", "testfile")
	if err != nil {
		t.Fatalf("Failed to create temporary file: %v", err)
	}
	defer func() { _ = os.Remove(tempFile.Name()) }()

	err = validateMountPoint(tempFile.Name())

	if err != nil {
		t.Errorf("validateMountPoint(%s) returned an unexpected error: %v", tempFile.Name(), err)
	}
}

func TestContainsAccessMode(t *testing.T) {
	modes := []corev1.PersistentVolumeAccessMode{
		corev1.ReadWriteOnce,
		corev1.ReadWriteMany,
	}
	if !containsAccessMode(modes, corev1.ReadWriteOnce) {
		t.Error("Expected mode to be found")
	}
	if containsAccessMode(modes, corev1.ReadOnlyMany) {
		t.Error("Did not expect mode to be found")
	}
}

func TestGeneratePodNameAndPort(t *testing.T) {
	name1, port1 := generatePodNameAndPort()
	name2, port2 := generatePodNameAndPort()
	if name1 == name2 {
		t.Error("Expected different pod names")
	}
	if port1 == port2 {
		t.Error("Expected different ports")
	}
}

func TestGeneratePodNameAndPort_Format(t *testing.T) {
	name, port := generatePodNameAndPort()
	if len(name) < len("volume-exposer-") {
		t.Errorf("Expected pod name to start with 'volume-exposer-', got '%s'", name)
	}
	if port < 1024 || port > 65535 {
		t.Errorf("Expected port in valid range (1024-65535), got %d", port)
	}
}

func TestCreatePodSpec(t *testing.T) {
	podSpec := createPodSpec("test-pod", 12345, "test-pvc", "publicKey", false, "whatever", "secret", "300m")
	if podSpec.Name != "test-pod" {
		t.Errorf("Expected pod name 'test-pod', got '%s'", podSpec.Name)
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

func TestCreatePodSpec_Comprehensive(t *testing.T) {
	t.Run("With image secret", func(t *testing.T) {
		podSpec := createPodSpec("test-pod", 12345, "test-pvc", "publicKey", false, "", "my-secret", "")
		if len(podSpec.Spec.ImagePullSecrets) == 0 {
			t.Error("Expected image pull secrets to be set")
		}
		if podSpec.Spec.ImagePullSecrets[0].Name != "my-secret" {
			t.Errorf("Expected image pull secret 'my-secret', got '%s'", podSpec.Spec.ImagePullSecrets[0].Name)
		}
	})

	t.Run("All pods attach PVC", func(t *testing.T) {
		podSpec := createPodSpec("test-pod", 12345, "test-pvc", "publicKey", false, "", "", "")
		if len(podSpec.Spec.Volumes) != 1 {
			t.Errorf("Expected 1 volume, got %d", len(podSpec.Spec.Volumes))
		}
	})

	t.Run("Root mode", func(t *testing.T) {
		podSpec := createPodSpec("test-pod", 12345, "test-pvc", "publicKey", true, "", "", "")
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
		podSpec := createPodSpec("test-pod", 12345, "test-pvc", "publicKey", false, customImage, "", "")
		if podSpec.Spec.Containers[0].Image != customImage {
			t.Errorf("Expected custom image '%s', got '%s'", customImage, podSpec.Spec.Containers[0].Image)
		}
	})

	t.Run("CPU limit", func(t *testing.T) {
		cpuLimit := "500m"
		podSpec := createPodSpec("test-pod", 12345, "test-pvc", "publicKey", false, "", "", cpuLimit)
		if podSpec.Spec.Containers[0].Resources.Limits.Cpu().String() != cpuLimit {
			t.Errorf("Expected CPU limit '%s', got '%s'", cpuLimit, podSpec.Spec.Containers[0].Resources.Limits.Cpu().String())
		}
	})

	t.Run("Default image", func(t *testing.T) {
		podSpec := createPodSpec("test-pod", 12345, "test-pvc", "publicKey", false, "", "", "")
		if podSpec.Spec.Containers[0].Image != Image {
			t.Errorf("Expected default image '%s', got '%s'", Image, podSpec.Spec.Containers[0].Image)
		}
	})

	t.Run("Same image for root mode", func(t *testing.T) {
		podSpec := createPodSpec("test-pod", 12345, "test-pvc", "publicKey", true, "", "", "")
		if podSpec.Spec.Containers[0].Image != Image {
			t.Errorf("Expected same image '%s' for root mode, got '%s'", Image, podSpec.Spec.Containers[0].Image)
		}
	})

	t.Run("Environment variables", func(t *testing.T) {
		podSpec := createPodSpec("test-pod", 12345, "test-pvc", "testPublicKey", false, "", "", "")
		envMap := make(map[string]string)
		for _, env := range podSpec.Spec.Containers[0].Env {
			envMap[env.Name] = env.Value
		}
		if envMap["SSH_PUBLIC_KEY"] != "testPublicKey" {
			t.Errorf("Expected SSH_PUBLIC_KEY='testPublicKey', got '%s'", envMap["SSH_PUBLIC_KEY"])
		}
		if envMap["NEEDS_ROOT"] != "false" {
			t.Errorf("Expected NEEDS_ROOT='false', got '%s'", envMap["NEEDS_ROOT"])
		}
	})
}

func TestBuildContainer(t *testing.T) {
	t.Run("Standard container", func(t *testing.T) {
		container := buildContainer("publicKey123", false, "", "")
		if container.Name != "volume-exposer" {
			t.Errorf("Expected container name 'volume-exposer', got '%s'", container.Name)
		}
		if container.Image != Image {
			t.Errorf("Expected image '%s', got '%s'", Image, container.Image)
		}
		if container.ImagePullPolicy != corev1.PullAlways {
			t.Error("Expected ImagePullPolicy to be PullAlways")
		}
		if len(container.Ports) != 1 || container.Ports[0].ContainerPort != int32(DefaultSSHPort) {
			t.Errorf("Expected container port %d", DefaultSSHPort)
		}
	})

	t.Run("Container with custom image and CPU limit", func(t *testing.T) {
		container := buildContainer("publicKey", true, "custom-image:latest", "1000m")
		if container.Image != "custom-image:latest" {
			t.Errorf("Expected custom image, got '%s'", container.Image)
		}
		cpuLimit := container.Resources.Limits.Cpu()
		if cpuLimit.IsZero() {
			t.Error("Expected CPU limit to be set")
		}
		if cpuLimit.Value() != 1 && cpuLimit.MilliValue() != 1000 {
			t.Errorf("Expected CPU limit to be 1 CPU or 1000m, got value=%d, millivalue=%d", cpuLimit.Value(), cpuLimit.MilliValue())
		}
	})
}

func TestBuildEnvVars(t *testing.T) {
	t.Run("Standard env vars", func(t *testing.T) {
		envVars := buildEnvVars("testKey", false)
		envMap := make(map[string]string)
		for _, env := range envVars {
			envMap[env.Name] = env.Value
		}
		if envMap["SSH_PUBLIC_KEY"] != "testKey" {
			t.Errorf("Expected SSH_PUBLIC_KEY='testKey', got '%s'", envMap["SSH_PUBLIC_KEY"])
		}
		if envMap["NEEDS_ROOT"] != "false" {
			t.Errorf("Expected NEEDS_ROOT='false', got '%s'", envMap["NEEDS_ROOT"])
		}
	})

	t.Run("Root env vars", func(t *testing.T) {
		envVars := buildEnvVars("rootKey", true)
		envMap := make(map[string]string)
		for _, env := range envVars {
			envMap[env.Name] = env.Value
		}
		if envMap["NEEDS_ROOT"] != "true" {
			t.Errorf("Expected NEEDS_ROOT='true', got '%s'", envMap["NEEDS_ROOT"])
		}
	})

	t.Run("No ROLE env var", func(t *testing.T) {
		envVars := buildEnvVars("key", false)
		for _, env := range envVars {
			if env.Name == "ROLE" {
				t.Error("Expected no ROLE env var")
			}
		}
	})
}

func TestSelectImage(t *testing.T) {
	t.Run("Custom image provided", func(t *testing.T) {
		img := selectImage("my-custom-image:v1")
		if img != "my-custom-image:v1" {
			t.Errorf("Expected 'my-custom-image:v1', got '%s'", img)
		}
	})

	t.Run("Custom image overrides needsRoot", func(t *testing.T) {
		img := selectImage("my-custom-image:v1")
		if img != "my-custom-image:v1" {
			t.Errorf("Expected 'my-custom-image:v1', got '%s'", img)
		}
	})

	t.Run("Default image when needsRoot is false", func(t *testing.T) {
		img := selectImage("")
		if img != Image {
			t.Errorf("Expected '%s', got '%s'", Image, img)
		}
	})

	t.Run("Same image when needsRoot is true", func(t *testing.T) {
		img := selectImage("")
		if img != Image {
			t.Errorf("Expected '%s', got '%s'", Image, img)
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
	t.Run("Labels for pod", func(t *testing.T) {
		labels := buildPodLabels("my-pvc", 12345)
		if labels["app"] != "volume-exposer" {
			t.Errorf("Expected app label 'volume-exposer', got '%s'", labels["app"])
		}
		if labels["pvcName"] != "my-pvc" {
			t.Errorf("Expected pvcName label 'my-pvc', got '%s'", labels["pvcName"])
		}
		if labels["portNumber"] != "12345" {
			t.Errorf("Expected portNumber label '12345', got '%s'", labels["portNumber"])
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

		if _, err := os.Stat(keyFilePath); os.IsNotExist(err) {
			t.Error("Expected temp key file to exist")
		}

		info, err := os.Stat(keyFilePath)
		if err != nil {
			t.Fatalf("Failed to stat key file: %v", err)
		}
		if info.Mode().Perm() != 0600 {
			t.Errorf("Expected file permissions 0600, got %o", info.Mode().Perm())
		}

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
		tempKeyFilesMu.Lock()
		tempKeyFiles = make(map[string]struct{})
		tempKeyFilesMu.Unlock()

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

		registerTempKeyFile(path1)
		registerTempKeyFile(path2)

		if _, err := os.Stat(path1); os.IsNotExist(err) {
			t.Error("Expected first temp file to exist")
		}
		if _, err := os.Stat(path2); os.IsNotExist(err) {
			t.Error("Expected second temp file to exist")
		}

		cleanupTempKeyFiles()

		if _, err := os.Stat(path1); !os.IsNotExist(err) {
			t.Error("Expected first temp file to be removed")
		}
		if _, err := os.Stat(path2); !os.IsNotExist(err) {
			t.Error("Expected second temp file to be removed")
		}
	})
}

func TestBuildEphemeralContainerSpec(t *testing.T) {
	t.Run("Build ephemeral container spec", func(t *testing.T) {
		spec := buildEphemeralContainerSpec(
			"test-ephemeral",
			"volume-name",
			"publicKey456",
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

		envMap := make(map[string]string)
		for _, env := range spec.Env {
			envMap[env.Name] = env.Value
		}
		if envMap["SSH_PUBLIC_KEY"] != "publicKey456" {
			t.Errorf("Expected SSH_PUBLIC_KEY='publicKey456', got '%s'", envMap["SSH_PUBLIC_KEY"])
		}
		if envMap["NEEDS_ROOT"] != "false" {
			t.Errorf("Expected NEEDS_ROOT='false', got '%s'", envMap["NEEDS_ROOT"])
		}
		// Verify removed env vars are not present
		if _, ok := envMap["ROLE"]; ok {
			t.Error("Expected no ROLE env var")
		}
		if _, ok := envMap["SSH_PRIVATE_KEY"]; ok {
			t.Error("Expected no SSH_PRIVATE_KEY env var")
		}
		if _, ok := envMap["PROXY_POD_IP"]; ok {
			t.Error("Expected no PROXY_POD_IP env var")
		}

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
			"publicKey",
			true,
			"",
		)

		if spec.Image != Image {
			t.Errorf("Expected image '%s', got '%s'", Image, spec.Image)
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
			"publicKey",
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
		ready, err := checkEphemeralContainerStatus(pod, "ephemeral-test", deadline, false)
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
		ready, err := checkEphemeralContainerStatus(pod, "ephemeral-test", deadline, false)
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
		ready, err := checkEphemeralContainerStatus(pod, "ephemeral-test", deadline, false)
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
		ready, err := checkEphemeralContainerStatus(pod, "ephemeral-test", deadline, false)
		if err == nil {
			t.Error("Expected error when ephemeral container is terminated")
		}
		if ready {
			t.Error("Expected ready to be false when ephemeral container is terminated")
		}
	})

	t.Run("No ephemeral container statuses with debug near deadline", func(t *testing.T) {
		nearDeadline := time.Now().Add(3 * time.Second)
		pod := &corev1.Pod{
			Status: corev1.PodStatus{
				EphemeralContainerStatuses: []corev1.ContainerStatus{},
			},
		}
		ready, err := checkEphemeralContainerStatus(pod, "ephemeral-test", nearDeadline, true)
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
		ready, err := checkEphemeralContainerStatus(pod, "ephemeral-test", deadline, true)
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
		ready, err := checkEphemeralContainerStatus(pod, "ephemeral-test", deadline, true)
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}
		if ready {
			t.Error("Expected ready to be false when ephemeral container is waiting")
		}
	})
}
