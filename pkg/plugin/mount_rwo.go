package plugin

import (
	"context"

	"k8s.io/client-go/kubernetes"
)

// handleRWO handles mounting of RWO (ReadWriteOnce) volumes that are already mounted.
func handleRWO(ctx context.Context, clientset *kubernetes.Clientset, namespace, pvcName, localMountPoint string, podUsingPVC string, needsRoot, debug bool, image, imageSecret, cpuLimit string) error {
	privateKey, publicKey, err := generateAndDebugKeys(debug)
	if err != nil {
		return err
	}

	proxyPodName, port, err := setupProxyPod(ctx, clientset, namespace, pvcName, publicKey, podUsingPVC, needsRoot, image, imageSecret, cpuLimit)
	if err != nil {
		return err
	}

	if err := setupEphemeralContainerWithTunnel(ctx, clientset, namespace, podUsingPVC, proxyPodName, privateKey, publicKey, needsRoot, debug, image); err != nil {
		return err
	}

	return setupPortForwardAndMount(ctx, namespace, proxyPodName, port, localMountPoint, pvcName, privateKey, needsRoot, debug, true)
}

// setupProxyPod creates a proxy pod for RWO volume access.
func setupProxyPod(ctx context.Context, clientset *kubernetes.Clientset, namespace, pvcName, publicKey, originalPodName string, needsRoot bool, image, imageSecret, cpuLimit string) (string, int, error) {
	config := mountConfig{
		role:            "proxy",
		sshPort:         ProxySSHPort,
		originalPodName: originalPodName,
	}
	return setupPodAndWait(ctx, clientset, namespace, pvcName, publicKey, config, needsRoot, image, imageSecret, cpuLimit)
}

// setupEphemeralContainerWithTunnel creates an ephemeral container and establishes SSH tunnel.
func setupEphemeralContainerWithTunnel(ctx context.Context, clientset *kubernetes.Clientset, namespace, podUsingPVC, proxyPodName, privateKey, publicKey string, needsRoot, debug bool, image string) error {
	proxyPodIP, err := getPodIP(ctx, clientset, namespace, proxyPodName)
	if err != nil {
		return err
	}

	if err := createEphemeralContainer(ctx, clientset, namespace, podUsingPVC, privateKey, publicKey, proxyPodIP, needsRoot, image); err != nil {
		return err
	}

	return waitForEphemeralContainerReady(ctx, clientset, namespace, podUsingPVC, debug)
}
