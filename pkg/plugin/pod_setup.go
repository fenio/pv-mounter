package plugin

import (
	"context"
	"fmt"
	"strconv"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
)

// setupPod creates a new pod for exposing a PVC via SSH.
func setupPod(ctx context.Context, clientset *kubernetes.Clientset, namespace, pvcName, publicKey string, needsRoot bool, image, imageSecret, cpuLimit string) (string, int, error) {
	podName, port := generatePodNameAndPort()
	pod := createPodSpec(podName, port, pvcName, publicKey, needsRoot, image, imageSecret, cpuLimit)
	if _, err := clientset.CoreV1().Pods(namespace).Create(ctx, pod, metav1.CreateOptions{}); err != nil {
		return "", 0, fmt.Errorf("failed to create pod: %w", err)
	}
	fmt.Printf("Pod %s created successfully\n", podName)
	return podName, port, nil
}

// waitForPodReady waits for a pod to reach the Ready state.
func waitForPodReady(ctx context.Context, clientset *kubernetes.Clientset, namespace, podName string) error {
	return wait.PollUntilContextTimeout(ctx, time.Second, 5*time.Minute, true, func(ctx context.Context) (bool, error) {
		pod, err := clientset.CoreV1().Pods(namespace).Get(ctx, podName, metav1.GetOptions{})
		if err != nil {
			return false, err
		}
		for _, cond := range pod.Status.Conditions {
			if cond.Type == corev1.PodReady && cond.Status == corev1.ConditionTrue {
				return true, nil
			}
		}
		return false, nil
	})
}

// createPodSpec creates the pod specification for volume exposer.
func createPodSpec(podName string, port int, pvcName, publicKey string, needsRoot bool, image, imageSecret, cpuLimit string) *corev1.Pod {
	container := buildContainer(publicKey, needsRoot, image, cpuLimit)
	labels := buildPodLabels(pvcName, port)
	imagePullSecrets := buildImagePullSecrets(imageSecret)
	podSecurityContext := buildPodSecurityContext(needsRoot)

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:   podName,
			Labels: labels,
		},
		Spec: corev1.PodSpec{
			Containers:       []corev1.Container{container},
			SecurityContext:  podSecurityContext,
			ImagePullSecrets: imagePullSecrets,
		},
	}

	attachPVCToPod(pod, pvcName)

	return pod
}

// buildContainer creates the container specification for volume exposer.
func buildContainer(publicKey string, needsRoot bool, image, cpuLimit string) corev1.Container {
	envVars := buildEnvVars(publicKey, needsRoot)
	imageToUse := selectImage(image)
	resources := buildResourceRequirements(cpuLimit)

	return corev1.Container{
		Name:            "volume-exposer",
		Image:           imageToUse,
		ImagePullPolicy: corev1.PullAlways,
		Ports: []corev1.ContainerPort{
			{ContainerPort: int32(DefaultSSHPort)},
		},
		Env:             envVars,
		SecurityContext: getSecurityContext(needsRoot),
		Resources:       resources,
	}
}

// buildEnvVars creates environment variables for the container.
func buildEnvVars(publicKey string, needsRoot bool) []corev1.EnvVar {
	return []corev1.EnvVar{
		{Name: "SSH_PUBLIC_KEY", Value: publicKey},
		{Name: "NEEDS_ROOT", Value: strconv.FormatBool(needsRoot)},
	}
}

// selectImage selects the appropriate container image.
func selectImage(image string) string {
	if image != "" {
		return image
	}
	return Image
}

// buildResourceRequirements creates resource requests and limits for the container.
func buildResourceRequirements(cpuLimit string) corev1.ResourceRequirements {
	resources := corev1.ResourceRequirements{
		Requests: corev1.ResourceList{
			corev1.ResourceCPU:              resource.MustParse(CPURequest),
			corev1.ResourceMemory:           resource.MustParse(MemoryRequest),
			corev1.ResourceEphemeralStorage: resource.MustParse(EphemeralStorageRequest),
		},
		Limits: corev1.ResourceList{
			corev1.ResourceMemory:           resource.MustParse(MemoryLimit),
			corev1.ResourceEphemeralStorage: resource.MustParse(EphemeralStorageLimit),
		},
	}
	if cpuLimit != "" {
		resources.Limits[corev1.ResourceCPU] = resource.MustParse(cpuLimit)
	}
	return resources
}

// buildPodLabels creates labels for the pod.
func buildPodLabels(pvcName string, port int) map[string]string {
	return map[string]string{
		"app":        "volume-exposer",
		"pvcName":    pvcName,
		"portNumber": strconv.Itoa(port),
	}
}

// buildImagePullSecrets creates image pull secrets if specified.
func buildImagePullSecrets(imageSecret string) []corev1.LocalObjectReference {
	if imageSecret == "" {
		return nil
	}
	return []corev1.LocalObjectReference{{Name: imageSecret}}
}

// buildPodSecurityContext creates the pod security context.
func buildPodSecurityContext(needsRoot bool) *corev1.PodSecurityContext {
	if needsRoot {
		runAsNonRoot := false
		var runAsUser, runAsGroup int64 = 0, 0
		return &corev1.PodSecurityContext{
			RunAsNonRoot: &runAsNonRoot,
			RunAsUser:    &runAsUser,
			RunAsGroup:   &runAsGroup,
		}
	}

	runAsNonRoot := true
	runAsUser := DefaultUserGroup
	runAsGroup := DefaultUserGroup
	return &corev1.PodSecurityContext{
		RunAsNonRoot: &runAsNonRoot,
		RunAsUser:    &runAsUser,
		RunAsGroup:   &runAsGroup,
	}
}

// attachPVCToPod attaches a PVC to the pod specification.
func attachPVCToPod(pod *corev1.Pod, pvcName string) {
	pod.Spec.Containers[0].VolumeMounts = []corev1.VolumeMount{
		{MountPath: "/volume", Name: "my-pvc"},
	}
	pod.Spec.Volumes = []corev1.Volume{
		{
			Name: "my-pvc",
			VolumeSource: corev1.VolumeSource{
				PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
					ClaimName: pvcName,
				},
			},
		},
	}
}

// getSecurityContext creates the container security context.
func getSecurityContext(needsRoot bool) *corev1.SecurityContext {
	if needsRoot {
		return getRootSecurityContext()
	}
	return getNonRootSecurityContext()
}

// getRootSecurityContext returns the security context for root access.
func getRootSecurityContext() *corev1.SecurityContext {
	allowPrivilegeEscalation := true
	readOnlyRootFilesystem := true
	seccompProfile := corev1.SeccompProfile{
		Type: corev1.SeccompProfileTypeRuntimeDefault,
	}
	return &corev1.SecurityContext{
		AllowPrivilegeEscalation: &allowPrivilegeEscalation,
		ReadOnlyRootFilesystem:   &readOnlyRootFilesystem,
		Capabilities: &corev1.Capabilities{
			Add: []corev1.Capability{"SYS_ADMIN", "SYS_CHROOT"},
		},
		SeccompProfile: &seccompProfile,
	}
}

// getNonRootSecurityContext returns the security context for non-root access.
func getNonRootSecurityContext() *corev1.SecurityContext {
	allowPrivilegeEscalation := false
	readOnlyRootFilesystem := true
	runAsNonRoot := true
	seccompProfile := corev1.SeccompProfile{
		Type: corev1.SeccompProfileTypeRuntimeDefault,
	}
	return &corev1.SecurityContext{
		AllowPrivilegeEscalation: &allowPrivilegeEscalation,
		ReadOnlyRootFilesystem:   &readOnlyRootFilesystem,
		Capabilities: &corev1.Capabilities{
			Drop: []corev1.Capability{"ALL"},
		},
		SeccompProfile: &seccompProfile,
		RunAsUser:      &DefaultID,
		RunAsGroup:     &DefaultID,
		RunAsNonRoot:   &runAsNonRoot,
	}
}

// getPVCVolumeName finds the volume name for a PVC in a pod.
func getPVCVolumeName(pod *corev1.Pod) (string, error) {
	for _, volume := range pod.Spec.Volumes {
		if volume.PersistentVolumeClaim != nil && volume.PersistentVolumeClaim.ClaimName != "" {
			return volume.Name, nil
		}
	}
	return "", fmt.Errorf("failed to find volume name in the existing pod")
}
