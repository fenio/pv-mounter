package plugin

import (
	"context"

	"k8s.io/client-go/kubernetes"
)

// handleRWONFS handles mounting of RWO (ReadWriteOnce) volumes that are already mounted, via NFS.
func handleRWONFS(ctx context.Context, clientset *kubernetes.Clientset, namespace, pvcName, localMountPoint string, podUsingPVC string, debug bool, image string) error {
	// Generate random local port for port-forward
	_, localPort := generatePodNameAndPort()

	// Inject NFS ephemeral container
	if err := createNFSEphemeralContainer(ctx, clientset, namespace, podUsingPVC, image); err != nil {
		return err
	}
	if err := waitForEphemeralContainerReady(ctx, clientset, namespace, podUsingPVC, debug); err != nil {
		return err
	}

	// Port-forward directly to workload pod and mount via NFS
	return setupNFSPortForwardAndMount(ctx, namespace, podUsingPVC, localPort, localMountPoint, pvcName, debug)
}
