package plugin

import (
	"context"

	"k8s.io/client-go/kubernetes"
)

// handleRWX handles mounting of RWX (ReadWriteMany) or unmounted RWO volumes.
func handleRWX(ctx context.Context, clientset *kubernetes.Clientset, namespace, pvcName, localMountPoint string, needsRoot, debug bool, image, imageSecret, cpuLimit string) error {
	privateKey, publicKey, err := generateAndDebugKeys(debug)
	if err != nil {
		return err
	}

	podName, port, err := setupPodAndWait(ctx, clientset, namespace, pvcName, publicKey, needsRoot, image, imageSecret, cpuLimit)
	if err != nil {
		return err
	}

	return setupPortForwardAndMount(ctx, namespace, podName, port, localMountPoint, pvcName, privateKey, needsRoot, debug)
}
