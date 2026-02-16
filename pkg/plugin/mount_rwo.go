package plugin

import (
	"context"

	"k8s.io/client-go/kubernetes"
)

// handleRWO handles mounting of RWO (ReadWriteOnce) volumes that are already mounted.
func handleRWO(ctx context.Context, clientset *kubernetes.Clientset, namespace, pvcName, localMountPoint string, podUsingPVC string, needsRoot, debug bool, image string) error {
	privateKey, publicKey, err := generateAndDebugKeys(debug)
	if err != nil {
		return err
	}

	// Generate random local port for port-forward
	_, localPort := generatePodNameAndPort()

	// Inject ephemeral container with sshd (uses DefaultSSHPort 2137)
	containerName, err := createEphemeralContainer(ctx, clientset, namespace, podUsingPVC, publicKey, needsRoot, image)
	if err != nil {
		return err
	}
	if err := waitForEphemeralContainerReady(ctx, clientset, namespace, podUsingPVC, containerName, debug); err != nil {
		return err
	}

	// Port-forward directly to workload pod and mount
	return setupPortForwardAndMount(ctx, namespace, podUsingPVC, localPort, localMountPoint, pvcName, privateKey, needsRoot, debug)
}
