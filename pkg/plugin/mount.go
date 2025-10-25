package plugin

import (
	"context"
	"crypto/elliptic"
	crand "crypto/rand"
	"encoding/json"
	"fmt"
	"math/big"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"sync"
	"syscall"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
)

const (
	// ImageVersion specifies the container image version for volume-exposer
	ImageVersion = "e4782f31d6"

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
	tempKeyFiles   = make(map[string]struct{})
	tempKeyFilesMu sync.Mutex
	cleanupOnce    sync.Once
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

func cleanupTempKeyFiles() {
	tempKeyFilesMu.Lock()
	defer tempKeyFilesMu.Unlock()
	for file := range tempKeyFiles {
		_ = os.Remove(file)
	}
}

func registerTempKeyFile(path string) {
	tempKeyFilesMu.Lock()
	defer tempKeyFilesMu.Unlock()
	tempKeyFiles[path] = struct{}{}
}

func unregisterTempKeyFile(path string) {
	tempKeyFilesMu.Lock()
	defer tempKeyFilesMu.Unlock()
	delete(tempKeyFiles, path)
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

func validateMountPoint(localMountPoint string) error {
	if _, err := os.Stat(localMountPoint); os.IsNotExist(err) {
		return fmt.Errorf("local mount point %s does not exist", localMountPoint)
	}
	return nil
}

type mountConfig struct {
	role            string
	sshPort         int
	originalPodName string
}

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

func setupPortForwardAndMount(ctx context.Context, namespace, podName string, port int, localMountPoint, pvcName, privateKey string, needsRoot bool) error {
	pfCmd, err := setupPortForwarding(ctx, namespace, podName, port)
	if err != nil {
		return err
	}
	if err := mountPVCOverSSH(ctx, port, localMountPoint, pvcName, privateKey, needsRoot); err != nil {
		cleanupPortForward(pfCmd)
		return err
	}
	return nil
}

func handleRWX(ctx context.Context, clientset *kubernetes.Clientset, namespace, pvcName, localMountPoint string, needsRoot, debug bool, image, imageSecret, cpuLimit string) error {
	privateKey, publicKey, err := generateAndDebugKeys(debug)
	if err != nil {
		return err
	}

	config := mountConfig{
		role:            "standalone",
		sshPort:         DefaultSSHPort,
		originalPodName: "",
	}

	podName, port, err := setupPodAndWait(ctx, clientset, namespace, pvcName, publicKey, config, needsRoot, image, imageSecret, cpuLimit)
	if err != nil {
		return err
	}

	if err := setupPortForwardAndMount(ctx, namespace, podName, port, localMountPoint, pvcName, privateKey, needsRoot); err != nil {
		cleanupTempKeyFiles()
		return err
	}

	return nil
}

func handleRWO(ctx context.Context, clientset *kubernetes.Clientset, namespace, pvcName, localMountPoint string, podUsingPVC string, needsRoot, debug bool, image, imageSecret, cpuLimit string) error {
	privateKey, publicKey, err := generateAndDebugKeys(debug)
	if err != nil {
		return err
	}

	config := mountConfig{
		role:            "proxy",
		sshPort:         ProxySSHPort,
		originalPodName: podUsingPVC,
	}

	podName, port, err := setupPodAndWait(ctx, clientset, namespace, pvcName, publicKey, config, needsRoot, image, imageSecret, cpuLimit)
	if err != nil {
		return err
	}

	proxyPodIP, err := getPodIP(ctx, clientset, namespace, podName)
	if err != nil {
		return err
	}

	if err := createEphemeralContainer(ctx, clientset, namespace, podUsingPVC, privateKey, publicKey, proxyPodIP, needsRoot, image); err != nil {
		return err
	}

	if err := setupPortForwardAndMount(ctx, namespace, podName, port, localMountPoint, pvcName, privateKey, needsRoot); err != nil {
		cleanupTempKeyFiles()
		return err
	}

	return nil
}

func cleanupPortForward(cmd *exec.Cmd) {
	if cmd != nil && cmd.Process != nil {
		_ = cmd.Process.Kill()
	}
}

func waitForPortAvailability(ctx context.Context, port int, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	dialer := &net.Dialer{
		Timeout: 500 * time.Millisecond,
	}
	for time.Now().Before(deadline) {
		conn, err := dialer.DialContext(ctx, "tcp", fmt.Sprintf("localhost:%d", port))
		if err == nil {
			_ = conn.Close()
			return nil
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(500 * time.Millisecond):
		}
	}
	return fmt.Errorf("timeout waiting for port %d to become available", port)
}

func createEphemeralContainer(ctx context.Context, clientset *kubernetes.Clientset, namespace, podName, privateKey, publicKey, proxyPodIP string, needsRoot bool, image string) error {
	existingPod, err := clientset.CoreV1().Pods(namespace).Get(ctx, podName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get existing pod: %w", err)
	}
	volumeName, err := getPVCVolumeName(existingPod)
	if err != nil {
		return err
	}
	ephemeralContainerName := fmt.Sprintf("volume-exposer-ephemeral-%s", randSeq(5))
	fmt.Printf("Adding ephemeral container %s to pod %s with volume name %s\n", ephemeralContainerName, podName, volumeName)
	imageToUse := image
	if imageToUse == "" {
		if needsRoot {
			imageToUse = PrivilegedImage
		} else {
			imageToUse = Image
		}
	}
	securityContext := getSecurityContext(needsRoot)
	ephemeralContainer := corev1.EphemeralContainer{
		EphemeralContainerCommon: corev1.EphemeralContainerCommon{
			Name:            ephemeralContainerName,
			Image:           imageToUse,
			ImagePullPolicy: corev1.PullAlways,
			Env: []corev1.EnvVar{
				{Name: "ROLE", Value: "ephemeral"},
				{Name: "SSH_PRIVATE_KEY", Value: privateKey},
				{Name: "PROXY_POD_IP", Value: proxyPodIP},
				{Name: "SSH_PUBLIC_KEY", Value: publicKey},
				{Name: "NEEDS_ROOT", Value: fmt.Sprintf("%v", needsRoot)},
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
	fmt.Printf("Successfully added ephemeral container %s to pod %s\n", ephemeralContainerName, podName)
	return nil
}

func getPodIP(ctx context.Context, clientset kubernetes.Interface, namespace, podName string) (string, error) {
	pod, err := clientset.CoreV1().Pods(namespace).Get(ctx, podName, metav1.GetOptions{})
	if err != nil {
		return "", fmt.Errorf("failed to get pod IP: %w", err)
	}
	return pod.Status.PodIP, nil
}

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

func contains(modes []corev1.PersistentVolumeAccessMode, modeToFind corev1.PersistentVolumeAccessMode) bool {
	for _, mode := range modes {
		if mode == modeToFind {
			return true
		}
	}
	return false
}

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

func setupPod(ctx context.Context, clientset *kubernetes.Clientset, namespace, pvcName, publicKey, role string, sshPort int, originalPodName string, needsRoot bool, image, imageSecret, cpuLimit string) (string, int, error) {
	podName, port := generatePodNameAndPort(role)
	pod := createPodSpec(podName, port, pvcName, publicKey, role, sshPort, originalPodName, needsRoot, image, imageSecret, cpuLimit)
	if _, err := clientset.CoreV1().Pods(namespace).Create(ctx, pod, metav1.CreateOptions{}); err != nil {
		return "", 0, fmt.Errorf("failed to create pod: %w", err)
	}
	fmt.Printf("Pod %s created successfully\n", podName)
	return podName, port, nil
}

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

func setupPortForwarding(ctx context.Context, namespace, podName string, port int) (*exec.Cmd, error) {
	cmd := exec.CommandContext(ctx, "kubectl", "port-forward", fmt.Sprintf("pod/%s", podName), fmt.Sprintf("%d:%d", port, DefaultSSHPort), "-n", namespace) // #nosec G204 -- namespace and podName are validated Kubernetes resource names
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start port-forward: %w", err)
	}

	if err := waitForPortAvailability(ctx, port, 30*time.Second); err != nil {
		cleanupPortForward(cmd)
		return nil, fmt.Errorf("port-forward failed to become available: %w", err)
	}

	return cmd, nil
}

func mountPVCOverSSH(
	ctx context.Context,
	port int,
	localMountPoint, pvcName, privateKey string,
	needsRoot bool) error {

	tmpFile, err := os.CreateTemp("", "ssh_key_*.pem")
	if err != nil {
		return fmt.Errorf("failed to create temporary file for SSH private key: %w", err)
	}
	keyFilePath := tmpFile.Name()
	registerTempKeyFile(keyFilePath)
	defer func() {
		_ = os.Remove(keyFilePath)
		unregisterTempKeyFile(keyFilePath)
	}()

	if err := os.Chmod(keyFilePath, 0600); err != nil {
		return fmt.Errorf("failed to set permissions on temporary SSH key file: %w", err)
	}

	if _, err := tmpFile.Write([]byte(privateKey)); err != nil {
		return fmt.Errorf("failed to write SSH private key to temporary file: %w", err)
	}
	if err := tmpFile.Close(); err != nil {
		return fmt.Errorf("failed to close temporary file: %w", err)
	}

	sshUser := "ve"
	if needsRoot {
		sshUser = "root"
	}

	sshfsCmd := exec.CommandContext(ctx, // #nosec G204 -- keyFilePath is a securely created temp file, localMountPoint is user-provided
		"sshfs",
		"-o", fmt.Sprintf("IdentityFile=%s", keyFilePath),
		"-o", "StrictHostKeyChecking=no",
		"-o", "UserKnownHostsFile=/dev/null",
		"-o", "nomap=ignore",
		fmt.Sprintf("%s@localhost:/volume", sshUser),
		localMountPoint,
		"-p", fmt.Sprintf("%d", port),
	)

	sshfsCmd.Stdout = os.Stdout
	sshfsCmd.Stderr = os.Stderr

	if err := sshfsCmd.Run(); err != nil {
		return fmt.Errorf("failed to mount PVC using SSHFS: %w", err)
	}

	fmt.Printf("PVC %s mounted successfully to %s\n", pvcName, localMountPoint)
	return nil
}

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

func createPodSpec(podName string, port int, pvcName, publicKey, role string, sshPort int, originalPodName string, needsRoot bool, image, imageSecret, cpuLimit string) *corev1.Pod {
	if sshPort < 0 || sshPort > 65535 {
		sshPort = DefaultSSHPort
	}
	envVars := []corev1.EnvVar{
		{Name: "SSH_PUBLIC_KEY", Value: publicKey},
		{Name: "SSH_PORT", Value: fmt.Sprintf("%d", sshPort)},
		{Name: "NEEDS_ROOT", Value: fmt.Sprintf("%v", needsRoot)},
	}
	if role == "standalone" || role == "proxy" {
		envVars = append(envVars, corev1.EnvVar{
			Name:  "ROLE",
			Value: role,
		})
	}
	imageToUse := image
	if imageToUse == "" {
		if needsRoot {
			imageToUse = PrivilegedImage
		} else {
			imageToUse = Image
		}
	}
	securityContext := getSecurityContext(needsRoot)
	runAsNonRoot := !needsRoot
	runAsUser := DefaultUserGroup
	runAsGroup := DefaultUserGroup
	if needsRoot {
		runAsUser = 0
		runAsGroup = 0
	}
	container := corev1.Container{
		Name:            "volume-exposer",
		Image:           imageToUse,
		ImagePullPolicy: corev1.PullAlways,
		Ports: []corev1.ContainerPort{
			{ContainerPort: int32(sshPort)}, // #nosec G115 -- sshPort is validated to be within valid port range (1024-65535)
		},
		Env:             envVars,
		SecurityContext: securityContext,
		Resources: corev1.ResourceRequirements{
			Requests: corev1.ResourceList{
				corev1.ResourceCPU:              resource.MustParse(CPURequest),
				corev1.ResourceMemory:           resource.MustParse(MemoryRequest),
				corev1.ResourceEphemeralStorage: resource.MustParse(EphemeralStorageRequest),
			},
			Limits: corev1.ResourceList{
				corev1.ResourceMemory:           resource.MustParse(MemoryLimit),
				corev1.ResourceEphemeralStorage: resource.MustParse(EphemeralStorageLimit),
			},
		},
	}
	if cpuLimit != "" {
		container.Resources.Limits[corev1.ResourceCPU] = resource.MustParse(cpuLimit)
	}
	labels := map[string]string{
		"app":        "volume-exposer",
		"pvcName":    pvcName,
		"portNumber": fmt.Sprintf("%d", port),
	}
	if originalPodName != "" {
		labels["originalPodName"] = originalPodName
	}
	imagePullSecrets := []corev1.LocalObjectReference{}
	if imageSecret != "" {
		imagePullSecrets = append(imagePullSecrets, corev1.LocalObjectReference{Name: imageSecret})
	}
	podSpec := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:   podName,
			Labels: labels,
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{container},
			SecurityContext: &corev1.PodSecurityContext{
				RunAsNonRoot: &runAsNonRoot,
				RunAsUser:    &runAsUser,
				RunAsGroup:   &runAsGroup,
			},
			ImagePullSecrets: imagePullSecrets,
		},
	}
	if role != "proxy" {
		container.VolumeMounts = []corev1.VolumeMount{
			{MountPath: "/volume", Name: "my-pvc"},
		}
		podSpec.Spec.Volumes = []corev1.Volume{
			{
				Name: "my-pvc",
				VolumeSource: corev1.VolumeSource{
					PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
						ClaimName: pvcName,
					},
				},
			},
		}
		podSpec.Spec.Containers[0] = container
	}
	return podSpec
}

func getPVCVolumeName(pod *corev1.Pod) (string, error) {
	for _, volume := range pod.Spec.Volumes {
		if volume.PersistentVolumeClaim != nil && volume.PersistentVolumeClaim.ClaimName != "" {
			return volume.Name, nil
		}
	}
	return "", fmt.Errorf("failed to find volume name in the existing pod")
}

func getSecurityContext(needsRoot bool) *corev1.SecurityContext {
	allowPrivilegeEscalationTrue := true
	allowPrivilegeEscalationFalse := false
	readOnlyRootFilesystemTrue := true
	runAsNonRootTrue := true
	seccompProfileRuntimeDefault := corev1.SeccompProfile{
		Type: corev1.SeccompProfileTypeRuntimeDefault,
	}
	if needsRoot {
		return &corev1.SecurityContext{
			AllowPrivilegeEscalation: &allowPrivilegeEscalationTrue,
			ReadOnlyRootFilesystem:   &readOnlyRootFilesystemTrue,
			Capabilities: &corev1.Capabilities{
				Add: []corev1.Capability{"SYS_ADMIN", "SYS_CHROOT"},
			},
			SeccompProfile: &seccompProfileRuntimeDefault,
		}
	}
	return &corev1.SecurityContext{
		AllowPrivilegeEscalation: &allowPrivilegeEscalationFalse,
		ReadOnlyRootFilesystem:   &readOnlyRootFilesystemTrue,
		Capabilities: &corev1.Capabilities{
			Drop: []corev1.Capability{"ALL"},
		},
		SeccompProfile: &seccompProfileRuntimeDefault,
		RunAsUser:      &DefaultID,
		RunAsGroup:     &DefaultID,
		RunAsNonRoot:   &runAsNonRootTrue,
	}
}
