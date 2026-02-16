package plugin

import (
	"context"
	"fmt"
	"strconv"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// setupNFSPod creates a new pod for exposing a PVC via NFS-Ganesha.
func setupNFSPod(ctx context.Context, clientset *kubernetes.Clientset, namespace, pvcName string, image, imageSecret, cpuLimit string) (string, int, error) {
	podName, port := generatePodNameAndPort()
	pod := createNFSPodSpec(podName, port, pvcName, image, imageSecret, cpuLimit)
	if _, err := clientset.CoreV1().Pods(namespace).Create(ctx, pod, metav1.CreateOptions{}); err != nil {
		return "", 0, fmt.Errorf("failed to create pod: %w", err)
	}
	fmt.Printf("Pod %s created successfully\n", podName)
	return podName, port, nil
}

// createNFSPodSpec creates the pod specification for NFS-Ganesha volume exposer.
func createNFSPodSpec(podName string, port int, pvcName, image, imageSecret, cpuLimit string) *corev1.Pod {
	container := buildNFSContainer(image, cpuLimit)
	labels := buildNFSPodLabels(pvcName, port)
	imagePullSecrets := buildImagePullSecrets(imageSecret)
	// NFS-Ganesha always needs root
	podSecurityContext := buildPodSecurityContext(true)

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

// buildNFSContainer creates the container specification for NFS-Ganesha.
func buildNFSContainer(image, cpuLimit string) corev1.Container {
	envVars := buildNFSEnvVars()
	imageToUse := selectNFSImage(image)
	resources := buildResourceRequirements(cpuLimit)

	return corev1.Container{
		Name:            "nfs-ganesha",
		Image:           imageToUse,
		ImagePullPolicy: corev1.PullAlways,
		Ports: []corev1.ContainerPort{
			{ContainerPort: int32(DefaultNFSPort)},
		},
		Env:             envVars,
		SecurityContext: getNFSSecurityContext(),
		Resources:       resources,
	}
}

// buildNFSEnvVars creates environment variables for the NFS container.
func buildNFSEnvVars() []corev1.EnvVar {
	return []corev1.EnvVar{
		{Name: "NEEDS_ROOT", Value: "true"},
		{Name: "LOG_LEVEL", Value: "EVENT"},
	}
}

// selectNFSImage selects the appropriate NFS container image.
func selectNFSImage(image string) string {
	if image != "" {
		return image
	}
	return NFSImage
}

// getNFSSecurityContext returns the security context for NFS-Ganesha container.
// No RunAsUser is set so ephemeral containers inherit the workload pod's user.
func getNFSSecurityContext() *corev1.SecurityContext {
	allowPrivilegeEscalation := true
	readOnlyRootFilesystem := false
	seccompProfile := corev1.SeccompProfile{
		Type: corev1.SeccompProfileTypeUnconfined,
	}
	return &corev1.SecurityContext{
		AllowPrivilegeEscalation: &allowPrivilegeEscalation,
		ReadOnlyRootFilesystem:   &readOnlyRootFilesystem,
		Capabilities: &corev1.Capabilities{
			Drop: []corev1.Capability{"ALL"},
			Add:  []corev1.Capability{"SYS_ADMIN", "DAC_READ_SEARCH", "DAC_OVERRIDE", "SYS_RESOURCE", "CHOWN", "FOWNER", "SETUID", "SETGID"},
		},
		SeccompProfile: &seccompProfile,
	}
}

// buildNFSPodLabels creates labels for the NFS pod.
func buildNFSPodLabels(pvcName string, port int) map[string]string {
	return map[string]string{
		"app":        "volume-exposer",
		"pvcName":    pvcName,
		"portNumber": strconv.Itoa(port),
		"backend":    "nfs",
	}
}

// createNFSEphemeralContainer creates an NFS ephemeral container in an existing pod.
func createNFSEphemeralContainer(ctx context.Context, clientset *kubernetes.Clientset, namespace, podName string, image string) (string, error) {
	existingPod, err := clientset.CoreV1().Pods(namespace).Get(ctx, podName, metav1.GetOptions{})
	if err != nil {
		return "", fmt.Errorf("failed to get existing pod: %w", err)
	}
	volumeName, err := getPVCVolumeName(existingPod)
	if err != nil {
		return "", err
	}
	ephemeralContainerName := fmt.Sprintf("nfs-ganesha-ephemeral-%s", randSeq(5))
	fmt.Printf("Adding ephemeral container %s to pod %s with volume name %s\n", ephemeralContainerName, podName, volumeName)

	ephemeralContainer := buildNFSEphemeralContainerSpec(ephemeralContainerName, volumeName, image)

	if err := patchPodWithEphemeralContainer(ctx, clientset, namespace, podName, ephemeralContainer); err != nil {
		return "", err
	}

	fmt.Printf("Successfully added ephemeral container %s to pod %s\n", ephemeralContainerName, podName)
	return ephemeralContainerName, nil
}

// buildNFSEphemeralContainerSpec creates the specification for an NFS ephemeral container.
// Uses a non-root user (65534/nobody) to comply with pods that have runAsNonRoot: true.
func buildNFSEphemeralContainerSpec(name, volumeName, image string) corev1.EphemeralContainer {
	imageToUse := selectNFSImage(image)
	securityContext := getNFSSecurityContext()
	// Ephemeral containers must respect the pod's runAsNonRoot policy.
	// Set a non-root UID; capabilities handle file access.
	nonRootUser := int64(65534)
	securityContext.RunAsUser = &nonRootUser

	return corev1.EphemeralContainer{
		EphemeralContainerCommon: corev1.EphemeralContainerCommon{
			Name:            name,
			Image:           imageToUse,
			ImagePullPolicy: corev1.PullAlways,
			Env:             buildNFSEnvVars(),
			SecurityContext: securityContext,
			VolumeMounts: []corev1.VolumeMount{
				{
					Name:      volumeName,
					MountPath: "/volume",
				},
			},
		},
	}
}
