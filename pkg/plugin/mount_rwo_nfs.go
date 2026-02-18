package plugin

import (
	"context"
	"fmt"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// handleRWONFS handles mounting of RWO (ReadWriteOnce) volumes that are already mounted, via NFS.
func handleRWONFS(ctx context.Context, clientset *kubernetes.Clientset, namespace, pvcName, localMountPoint string, podUsingPVC string, debug bool, image string) error {
	// Generate random local port for port-forward
	_, localPort := generatePodNameAndPort()

	// Check if there's already a running NFS ephemeral container we can reuse
	existing, err := findRunningNFSEphemeralContainer(ctx, clientset, namespace, podUsingPVC)
	if err != nil {
		return err
	}

	if existing != "" {
		fmt.Printf("Reusing existing NFS ephemeral container %s in pod %s\n", existing, podUsingPVC)
	} else {
		// Inject NFS ephemeral container
		containerName, err := createNFSEphemeralContainer(ctx, clientset, namespace, podUsingPVC, image)
		if err != nil {
			return err
		}
		if err := waitForEphemeralContainerReady(ctx, clientset, namespace, podUsingPVC, containerName, debug); err != nil {
			return err
		}
	}

	// Port-forward directly to workload pod and mount via NFS
	return setupNFSPortForwardAndMount(ctx, namespace, podUsingPVC, localPort, localMountPoint, pvcName, debug)
}

// findRunningNFSEphemeralContainer checks if the pod already has a running NFS ephemeral container.
func findRunningNFSEphemeralContainer(ctx context.Context, clientset *kubernetes.Clientset, namespace, podName string) (string, error) {
	pod, err := clientset.CoreV1().Pods(namespace).Get(ctx, podName, metav1.GetOptions{})
	if err != nil {
		return "", fmt.Errorf("failed to get pod: %w", err)
	}

	for _, status := range pod.Status.EphemeralContainerStatuses {
		if strings.HasPrefix(status.Name, "volume-exposer-") && status.State.Running != nil {
			return status.Name, nil
		}
	}
	return "", nil
}
