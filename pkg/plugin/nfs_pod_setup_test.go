package plugin

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
)

func TestCreateNFSPodSpec(t *testing.T) {
	t.Run("Basic NFS pod spec", func(t *testing.T) {
		podSpec := createNFSPodSpec("test-pod", 12345, "test-pvc", "", "", "")
		if podSpec.Name != "test-pod" {
			t.Errorf("Expected pod name 'test-pod', got '%s'", podSpec.Name)
		}
		if podSpec.Spec.Containers[0].Name != "nfs-ganesha" {
			t.Errorf("Expected container name 'nfs-ganesha', got '%s'", podSpec.Spec.Containers[0].Name)
		}
		if podSpec.Spec.Containers[0].Ports[0].ContainerPort != int32(DefaultNFSPort) {
			t.Errorf("Expected container port %d, got %d", DefaultNFSPort, podSpec.Spec.Containers[0].Ports[0].ContainerPort)
		}
	})

	t.Run("NFS pod always runs as root", func(t *testing.T) {
		podSpec := createNFSPodSpec("test-pod", 12345, "test-pvc", "", "", "")
		if *podSpec.Spec.SecurityContext.RunAsNonRoot != false {
			t.Error("Expected RunAsNonRoot to be false for NFS pod")
		}
		if *podSpec.Spec.SecurityContext.RunAsUser != 0 {
			t.Error("Expected RunAsUser to be 0 for NFS pod")
		}
		if *podSpec.Spec.SecurityContext.RunAsGroup != 0 {
			t.Error("Expected RunAsGroup to be 0 for NFS pod")
		}
	})
}

func TestBuildNFSPodLabels(t *testing.T) {
	t.Run("Labels include backend=nfs", func(t *testing.T) {
		labels := buildNFSPodLabels("my-pvc", 12345)
		if labels["app"] != "volume-exposer" {
			t.Errorf("Expected app label 'volume-exposer', got '%s'", labels["app"])
		}
		if labels["pvcName"] != "my-pvc" {
			t.Errorf("Expected pvcName label 'my-pvc', got '%s'", labels["pvcName"])
		}
		if labels["portNumber"] != "12345" {
			t.Errorf("Expected portNumber label '12345', got '%s'", labels["portNumber"])
		}
		if labels["backend"] != "nfs" {
			t.Errorf("Expected backend label 'nfs', got '%s'", labels["backend"])
		}
	})
}

func TestGetNFSSecurityContext(t *testing.T) {
	t.Run("NFS security context", func(t *testing.T) {
		secCtx := getNFSSecurityContext()
		if secCtx == nil {
			t.Fatal("Expected non-nil security context")
		}
		if secCtx.RunAsUser != nil {
			t.Error("Expected RunAsUser to be nil (inherit from pod)")
		}
		if secCtx.RunAsGroup != nil {
			t.Error("Expected RunAsGroup to be nil (inherit from pod)")
		}
		if *secCtx.AllowPrivilegeEscalation != true {
			t.Error("Expected AllowPrivilegeEscalation to be true")
		}
		if *secCtx.ReadOnlyRootFilesystem != false {
			t.Error("Expected ReadOnlyRootFilesystem to be false for NFS")
		}

		// Check capabilities
		if secCtx.Capabilities == nil {
			t.Fatal("Expected capabilities to be set")
		}
		if len(secCtx.Capabilities.Drop) != 1 || secCtx.Capabilities.Drop[0] != "ALL" {
			t.Errorf("Expected ALL capability to be dropped, got %v", secCtx.Capabilities.Drop)
		}

		requiredCaps := []corev1.Capability{"SYS_ADMIN", "DAC_READ_SEARCH", "DAC_OVERRIDE", "SYS_RESOURCE", "CHOWN", "FOWNER", "SETUID", "SETGID"}
		addedCaps := make(map[corev1.Capability]bool)
		for _, cap := range secCtx.Capabilities.Add {
			addedCaps[cap] = true
		}
		for _, cap := range requiredCaps {
			if !addedCaps[cap] {
				t.Errorf("Expected %s capability", cap)
			}
		}

		if secCtx.SeccompProfile == nil || secCtx.SeccompProfile.Type != corev1.SeccompProfileTypeUnconfined {
			t.Error("Expected SeccompProfile to be Unconfined")
		}
	})
}

func TestBuildNFSEphemeralContainerSpec(t *testing.T) {
	t.Run("Build NFS ephemeral container spec", func(t *testing.T) {
		spec := buildNFSEphemeralContainerSpec(
			"nfs-ganesha-ephemeral-test",
			"volume-name",
			"",
		)

		if spec.Name != "nfs-ganesha-ephemeral-test" {
			t.Errorf("Expected name 'nfs-ganesha-ephemeral-test', got '%s'", spec.Name)
		}
		if spec.Image != NFSImage {
			t.Errorf("Expected image '%s', got '%s'", NFSImage, spec.Image)
		}
		if spec.ImagePullPolicy != corev1.PullAlways {
			t.Error("Expected ImagePullPolicy to be PullAlways")
		}

		// Verify env vars - only NEEDS_ROOT, no SSH_PUBLIC_KEY
		envMap := make(map[string]string)
		for _, env := range spec.Env {
			envMap[env.Name] = env.Value
		}
		if _, ok := envMap["SSH_PUBLIC_KEY"]; ok {
			t.Error("Expected no SSH_PUBLIC_KEY env var for NFS")
		}
		if envMap["NEEDS_ROOT"] != "true" {
			t.Errorf("Expected NEEDS_ROOT='true', got '%s'", envMap["NEEDS_ROOT"])
		}

		// Verify volume mount
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

	t.Run("Build NFS ephemeral container spec with custom image", func(t *testing.T) {
		customImage := "my-custom-nfs:v2"
		spec := buildNFSEphemeralContainerSpec(
			"nfs-ganesha-ephemeral-custom",
			"volume-name",
			customImage,
		)

		if spec.Image != customImage {
			t.Errorf("Expected custom image '%s', got '%s'", customImage, spec.Image)
		}
	})
}

func TestSelectNFSImage(t *testing.T) {
	t.Run("Default NFS image", func(t *testing.T) {
		img := selectNFSImage("")
		if img != NFSImage {
			t.Errorf("Expected '%s', got '%s'", NFSImage, img)
		}
	})

	t.Run("Custom NFS image", func(t *testing.T) {
		img := selectNFSImage("my-custom-nfs:v1")
		if img != "my-custom-nfs:v1" {
			t.Errorf("Expected 'my-custom-nfs:v1', got '%s'", img)
		}
	})
}

func TestBuildNFSEnvVars(t *testing.T) {
	t.Run("NFS env vars", func(t *testing.T) {
		envVars := buildNFSEnvVars()
		envMap := make(map[string]string)
		for _, env := range envVars {
			envMap[env.Name] = env.Value
		}
		if envMap["NEEDS_ROOT"] != "true" {
			t.Errorf("Expected NEEDS_ROOT='true', got '%s'", envMap["NEEDS_ROOT"])
		}
		if envMap["LOG_LEVEL"] != "EVENT" {
			t.Errorf("Expected LOG_LEVEL='EVENT', got '%s'", envMap["LOG_LEVEL"])
		}
	})
}
