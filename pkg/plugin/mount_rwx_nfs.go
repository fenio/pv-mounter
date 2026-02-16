package plugin

import (
	"context"

	"k8s.io/client-go/kubernetes"
)

// handleRWXNFS handles mounting of RWX (ReadWriteMany) or unmounted RWO volumes via NFS.
func handleRWXNFS(ctx context.Context, clientset *kubernetes.Clientset, namespace, pvcName, localMountPoint string, debug bool, image, imageSecret, cpuLimit string) error {
	podName, port, err := setupNFSPod(ctx, clientset, namespace, pvcName, image, imageSecret, cpuLimit)
	if err != nil {
		return err
	}

	if err := waitForPodReady(ctx, clientset, namespace, podName); err != nil {
		return err
	}

	return setupNFSPortForwardAndMount(ctx, namespace, podName, port, localMountPoint, pvcName, debug)
}
