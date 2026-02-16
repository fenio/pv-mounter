package plugin

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
)

// createEphemeralContainer creates an ephemeral container in an existing pod.
func createEphemeralContainer(ctx context.Context, clientset *kubernetes.Clientset, namespace, podName, publicKey string, needsRoot bool, image string) (string, error) {
	existingPod, err := clientset.CoreV1().Pods(namespace).Get(ctx, podName, metav1.GetOptions{})
	if err != nil {
		return "", fmt.Errorf("failed to get existing pod: %w", err)
	}
	volumeName, err := getPVCVolumeName(existingPod)
	if err != nil {
		return "", err
	}
	ephemeralContainerName := fmt.Sprintf("volume-exposer-ephemeral-%s", randSeq(5))
	fmt.Printf("Adding ephemeral container %s to pod %s with volume name %s\n", ephemeralContainerName, podName, volumeName)

	ephemeralContainer := buildEphemeralContainerSpec(ephemeralContainerName, volumeName, publicKey, needsRoot, image)

	if err := patchPodWithEphemeralContainer(ctx, clientset, namespace, podName, ephemeralContainer); err != nil {
		return "", err
	}

	fmt.Printf("Successfully added ephemeral container %s to pod %s\n", ephemeralContainerName, podName)
	return ephemeralContainerName, nil
}

// buildEphemeralContainerSpec creates the specification for an ephemeral container.
func buildEphemeralContainerSpec(name, volumeName, publicKey string, needsRoot bool, image string) corev1.EphemeralContainer {
	imageToUse := selectImage(image)
	securityContext := getSecurityContext(needsRoot)

	return corev1.EphemeralContainer{
		EphemeralContainerCommon: corev1.EphemeralContainerCommon{
			Name:            name,
			Image:           imageToUse,
			ImagePullPolicy: corev1.PullAlways,
			Env: []corev1.EnvVar{
				{Name: "SSH_PUBLIC_KEY", Value: publicKey},
				{Name: "NEEDS_ROOT", Value: strconv.FormatBool(needsRoot)},
			},
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

// patchPodWithEphemeralContainer patches a pod to add an ephemeral container.
func patchPodWithEphemeralContainer(ctx context.Context, clientset *kubernetes.Clientset, namespace, podName string, ephemeralContainer corev1.EphemeralContainer) error {
	patchData, err := json.Marshal(map[string]interface{}{
		"spec": map[string]interface{}{
			"ephemeralContainers": []corev1.EphemeralContainer{ephemeralContainer},
		},
	})
	if err != nil {
		return fmt.Errorf("failed to marshal ephemeral container spec: %w", err)
	}
	_, err = clientset.CoreV1().Pods(namespace).Patch(ctx, podName, types.StrategicMergePatchType, patchData, metav1.PatchOptions{}, "ephemeralcontainers")
	if err != nil {
		return fmt.Errorf("failed to patch pod with ephemeral container: %w", err)
	}
	return nil
}

// waitForEphemeralContainerReady waits for an ephemeral container to be ready.
func waitForEphemeralContainerReady(ctx context.Context, clientset *kubernetes.Clientset, namespace, podName, containerName string, debug bool) error {
	timeout := 60 * time.Second
	deadline := time.Now().Add(timeout)

	if debug {
		fmt.Printf("Waiting for ephemeral container %s to be ready in pod %s...\n", containerName, podName)
	}

	return wait.PollUntilContextTimeout(ctx, time.Second, timeout, true, func(ctx context.Context) (bool, error) {
		pod, err := clientset.CoreV1().Pods(namespace).Get(ctx, podName, metav1.GetOptions{})
		if err != nil {
			return false, err
		}

		return checkEphemeralContainerStatus(pod, containerName, deadline, debug)
	})
}

// checkEphemeralContainerStatus checks the status of an ephemeral container.
func checkEphemeralContainerStatus(pod *corev1.Pod, containerName string, deadline time.Time, debug bool) (bool, error) {
	// Find the specific ephemeral container by name
	var ephemeralStatus *corev1.ContainerStatus
	for i := range pod.Status.EphemeralContainerStatuses {
		if pod.Status.EphemeralContainerStatuses[i].Name == containerName {
			ephemeralStatus = &pod.Status.EphemeralContainerStatuses[i]
			break
		}
	}

	if ephemeralStatus == nil {
		if debug && time.Now().Add(5*time.Second).After(deadline) {
			fmt.Printf("Still waiting for ephemeral container %s status to appear...\n", containerName)
		}
		return false, nil
	}

	if ephemeralStatus.State.Running != nil {
		if debug {
			fmt.Printf("Ephemeral container %s is running\n", ephemeralStatus.Name)
		}
		time.Sleep(3 * time.Second)
		return true, nil
	}

	if ephemeralStatus.State.Waiting != nil {
		if debug {
			fmt.Printf("Ephemeral container %s is waiting: %s\n", ephemeralStatus.Name, ephemeralStatus.State.Waiting.Reason)
		}
		return false, nil
	}

	if ephemeralStatus.State.Terminated != nil {
		return false, fmt.Errorf("ephemeral container terminated: %s", ephemeralStatus.State.Terminated.Reason)
	}

	return false, nil
}
