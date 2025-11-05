package plugin

import (
	"context"
	"crypto/elliptic"
	crand "crypto/rand"
	"fmt"
	"math/big"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

const (
	// ImageVersion specifies the container image version for volume-exposer
	ImageVersion = "70339aa201"

	// Image is the default non-privileged container image
	Image = "bfenski/volume-exposer:" + ImageVersion
	// PrivilegedImage is the privileged container image for root access
	PrivilegedImage = "bfenski/volume-exposer-privileged:" + ImageVersion
	// DefaultUserGroup is the default user and group ID
	DefaultUserGroup int64 = 2137
	// DefaultSSHPort is the default SSH port for the SSH server
	DefaultSSHPort int = 2137
	// ProxySSHPort is the SSH port used by proxy pods
	ProxySSHPort int = 6666

	// CPURequest is the default CPU request for containers
	CPURequest = "10m"
	// MemoryRequest is the default memory request
	MemoryRequest = "50Mi"
	// MemoryLimit is the default memory limit
	MemoryLimit = "100Mi"
	// EphemeralStorageRequest is the default ephemeral storage request
	EphemeralStorageRequest = "1Mi"
	// EphemeralStorageLimit is the default ephemeral storage limit
	EphemeralStorageLimit = "2Mi"
)

// DefaultID specifies the default user and group ID for the SSH user
var DefaultID int64 = 2137

var (
	cleanupOnce sync.Once
)

func init() {
	cleanupOnce.Do(func() {
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
		go func() {
			<-sigChan
			cleanupTempKeyFiles()
			os.Exit(1)
		}()
	})
}

// mountConfig holds configuration for mounting operations.
type mountConfig struct {
	role            string
	sshPort         int
	originalPodName string
}

// Mount establishes an SSHFS connection to mount a PVC to a local directory.
func Mount(ctx context.Context, namespace, pvcName, localMountPoint string, needsRoot, debug bool, image, imageSecret, cpuLimit string) error {
	checkSSHFS()
	if err := ValidateKubernetesName(namespace, "namespace"); err != nil {
		return err
	}
	if err := ValidateKubernetesName(pvcName, "pvc-name"); err != nil {
		return err
	}
	if err := validateMountPoint(localMountPoint); err != nil {
		return err
	}
	clientset, err := BuildKubeClient()
	if err != nil {
		return err
	}
	pvc, err := checkPVCUsage(ctx, clientset, namespace, pvcName)
	if err != nil {
		return err
	}
	canBeMounted, podUsingPVC, err := checkPVAccessMode(ctx, clientset, pvc, namespace)
	if err != nil {
		return err
	}
	if canBeMounted {
		return handleRWX(ctx, clientset, namespace, pvcName, localMountPoint, needsRoot, debug, image, imageSecret, cpuLimit)
	}
	return handleRWO(ctx, clientset, namespace, pvcName, localMountPoint, podUsingPVC, needsRoot, debug, image, imageSecret, cpuLimit)
}

// validateMountPoint checks if the local mount point exists.
func validateMountPoint(localMountPoint string) error {
	if _, err := os.Stat(localMountPoint); os.IsNotExist(err) {
		return fmt.Errorf("local mount point %s does not exist", localMountPoint)
	}
	return nil
}

// generateAndDebugKeys generates SSH key pair and optionally prints debug info.
func generateAndDebugKeys(debug bool) (privateKey, publicKey string, err error) {
	privateKey, publicKey, err = GenerateKeyPair(elliptic.P256())
	if err != nil {
		return "", "", fmt.Errorf("error generating key pair: %w", err)
	}
	if debug {
		fmt.Printf("Private Key:\n%s\n", privateKey)
	}
	return privateKey, publicKey, nil
}

// setupPodAndWait creates a pod and waits for it to be ready.
func setupPodAndWait(ctx context.Context, clientset *kubernetes.Clientset, namespace, pvcName, publicKey string, config mountConfig, needsRoot bool, image, imageSecret, cpuLimit string) (podName string, port int, err error) {
	podName, port, err = setupPod(ctx, clientset, namespace, pvcName, publicKey, config.role, config.sshPort, config.originalPodName, needsRoot, image, imageSecret, cpuLimit)
	if err != nil {
		return "", 0, err
	}
	if err := waitForPodReady(ctx, clientset, namespace, podName); err != nil {
		return "", 0, err
	}
	return podName, port, nil
}

// setupPortForwardAndMount establishes port forwarding and mounts the volume.
func setupPortForwardAndMount(ctx context.Context, namespace, podName string, port int, localMountPoint, pvcName, privateKey string, needsRoot, debug bool, isProxyMode bool) error {
	timeout := 30 * time.Second
	if isProxyMode {
		timeout = 60 * time.Second
	}
	pfCmd, err := setupPortForwarding(ctx, namespace, podName, port, debug, timeout)
	if err != nil {
		return err
	}
	if err := mountPVCOverSSH(ctx, port, localMountPoint, pvcName, privateKey, needsRoot); err != nil {
		cleanupPortForward(pfCmd)
		return err
	}
	return nil
}

// checkPVAccessMode checks the access mode of a PV and determines if it can be mounted.
func checkPVAccessMode(ctx context.Context, clientset *kubernetes.Clientset, pvc *corev1.PersistentVolumeClaim, namespace string) (bool, string, error) {
	pvName := pvc.Spec.VolumeName
	pv, err := clientset.CoreV1().PersistentVolumes().Get(ctx, pvName, metav1.GetOptions{})
	if err != nil {
		return true, "", fmt.Errorf("failed to get PV: %w", err)
	}

	if contains(pv.Spec.AccessModes, corev1.ReadWriteOnce) {
		podList, err := clientset.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{})
		if err != nil {
			return true, "", fmt.Errorf("failed to list pods: %w", err)
		}
		for _, pod := range podList.Items {
			for _, volume := range pod.Spec.Volumes {
				if volume.PersistentVolumeClaim != nil && volume.PersistentVolumeClaim.ClaimName == pvc.Name {
					return false, pod.Name, nil
				}
			}
		}
	}
	return true, "", nil
}

// contains checks if a slice of access modes contains a specific mode.
func contains(modes []corev1.PersistentVolumeAccessMode, modeToFind corev1.PersistentVolumeAccessMode) bool {
	for _, mode := range modes {
		if mode == modeToFind {
			return true
		}
	}
	return false
}

// checkPVCUsage verifies that a PVC exists and is bound.
func checkPVCUsage(ctx context.Context, clientset kubernetes.Interface, namespace, pvcName string) (*corev1.PersistentVolumeClaim, error) {
	pvc, err := clientset.CoreV1().PersistentVolumeClaims(namespace).Get(ctx, pvcName, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get PVC: %w", err)
	}
	if pvc.Status.Phase != corev1.ClaimBound {
		return nil, fmt.Errorf("PVC %s is not bound", pvcName)
	}
	return pvc, nil
}

// generatePodNameAndPort generates a unique pod name and random port.
func generatePodNameAndPort(role string) (string, int) {
	suffix := randSeq(5)
	baseName := "volume-exposer"
	if role == "proxy" {
		baseName = "volume-exposer-proxy"
	}
	podName := fmt.Sprintf("%s-%s", baseName, suffix)
	portBig, err := crand.Int(crand.Reader, big.NewInt(64511))
	if err != nil {
		return podName, 1024
	}
	port := int(portBig.Int64()) + 1024
	return podName, port
}
