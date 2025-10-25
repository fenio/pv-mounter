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

	return setupPortForwardAndMount(ctx, namespace, podName, port, localMountPoint, pvcName, privateKey, needsRoot, debug, false)
}

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

func setupProxyPod(ctx context.Context, clientset *kubernetes.Clientset, namespace, pvcName, publicKey, originalPodName string, needsRoot bool, image, imageSecret, cpuLimit string) (string, int, error) {
	config := mountConfig{
		role:            "proxy",
		sshPort:         ProxySSHPort,
		originalPodName: originalPodName,
	}
	return setupPodAndWait(ctx, clientset, namespace, pvcName, publicKey, config, needsRoot, image, imageSecret, cpuLimit)
}

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

func cleanupPortForward(cmd *exec.Cmd) {
	if cmd != nil && cmd.Process != nil {
		_ = cmd.Process.Kill()
	}
}

func waitForSSHReady(ctx context.Context, port int, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			if time.Now().After(deadline) {
				return fmt.Errorf("timeout waiting for SSH daemon to become ready on port %d", port)
			}

			if isSSHReady(ctx, port) {
				return nil
			}
		}
	}
}

func isSSHReady(ctx context.Context, port int) bool {
	dialer := &net.Dialer{Timeout: time.Second}
	conn, err := dialer.DialContext(ctx, "tcp", fmt.Sprintf("127.0.0.1:%d", port))
	if err != nil {
		return false
	}
	defer func() {
		_ = conn.Close()
	}()

	buf := make([]byte, 4)
	if err := conn.SetReadDeadline(time.Now().Add(2 * time.Second)); err != nil {
		return false
	}

	n, err := conn.Read(buf)
	return err == nil && n >= 3 && string(buf[:3]) == "SSH"
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

	ephemeralContainer := buildEphemeralContainerSpec(ephemeralContainerName, volumeName, privateKey, publicKey, proxyPodIP, needsRoot, image)

	if err := patchPodWithEphemeralContainer(ctx, clientset, namespace, podName, ephemeralContainer); err != nil {
		return err
	}

	fmt.Printf("Successfully added ephemeral container %s to pod %s\n", ephemeralContainerName, podName)
	return nil
}

func buildEphemeralContainerSpec(name, volumeName, privateKey, publicKey, proxyPodIP string, needsRoot bool, image string) corev1.EphemeralContainer {
	imageToUse := selectImage(image, needsRoot)
	securityContext := getSecurityContext(needsRoot)

	return corev1.EphemeralContainer{
		EphemeralContainerCommon: corev1.EphemeralContainerCommon{
			Name:            name,
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
}

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

func waitForEphemeralContainerReady(ctx context.Context, clientset *kubernetes.Clientset, namespace, podName string, debug bool) error {
	timeout := 60 * time.Second
	deadline := time.Now().Add(timeout)

	if debug {
		fmt.Printf("Waiting for ephemeral container to be ready in pod %s...\n", podName)
	}

	return wait.PollUntilContextTimeout(ctx, time.Second, timeout, true, func(ctx context.Context) (bool, error) {
		pod, err := clientset.CoreV1().Pods(namespace).Get(ctx, podName, metav1.GetOptions{})
		if err != nil {
			return false, err
		}

		return checkEphemeralContainerStatus(pod, deadline, debug)
	})
}

func checkEphemeralContainerStatus(pod *corev1.Pod, deadline time.Time, debug bool) (bool, error) {
	if len(pod.Status.EphemeralContainerStatuses) == 0 {
		if debug && time.Now().Add(5*time.Second).After(deadline) {
			fmt.Printf("Still waiting for ephemeral container status to appear...\n")
		}
		return false, nil
	}

	ephemeralStatus := pod.Status.EphemeralContainerStatuses[len(pod.Status.EphemeralContainerStatuses)-1]

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

func setupPortForwarding(ctx context.Context, namespace, podName string, port int, debug bool, timeout time.Duration) (*exec.Cmd, error) {
	cmd := exec.CommandContext(ctx, "kubectl", "port-forward", fmt.Sprintf("pod/%s", podName), fmt.Sprintf("%d:%d", port, DefaultSSHPort), "-n", namespace) // #nosec G204 -- namespace and podName are validated Kubernetes resource names
	if debug {
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
	}
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start port-forward: %w", err)
	}

	if err := waitForSSHReady(ctx, port, timeout); err != nil {
		cleanupPortForward(cmd)
		return nil, fmt.Errorf("failed to establish SSH connection: %w", err)
	}

	if !debug {
		fmt.Printf("Forwarding from 127.0.0.1:%d -> %d\n", port, DefaultSSHPort)
	}
	return cmd, nil
}

func mountPVCOverSSH(
	ctx context.Context,
	port int,
	localMountPoint, pvcName, privateKey string,
	needsRoot bool) error {

	keyFilePath, cleanup, err := createTempSSHKeyFile(privateKey)
	if err != nil {
		return err
	}
	defer cleanup()

	sshUser := selectSSHUser(needsRoot)
	sshfsCmd := buildSSHFSCommand(ctx, keyFilePath, sshUser, localMountPoint, port)

	sshfsCmd.Stdout = os.Stdout
	sshfsCmd.Stderr = os.Stderr

	if err := sshfsCmd.Run(); err != nil {
		return fmt.Errorf("failed to mount PVC using SSHFS: %w", err)
	}

	fmt.Printf("PVC %s mounted successfully to %s\n", pvcName, localMountPoint)
	return nil
}

func createTempSSHKeyFile(privateKey string) (string, func(), error) {
	tmpFile, err := os.CreateTemp("", "ssh_key_*.pem")
	if err != nil {
		return "", nil, fmt.Errorf("failed to create temporary file for SSH private key: %w", err)
	}
	keyFilePath := tmpFile.Name()
	registerTempKeyFile(keyFilePath)

	cleanup := func() {
		_ = os.Remove(keyFilePath)
		unregisterTempKeyFile(keyFilePath)
	}

	if err := os.Chmod(keyFilePath, 0600); err != nil {
		cleanup()
		return "", nil, fmt.Errorf("failed to set permissions on temporary SSH key file: %w", err)
	}

	if _, err := tmpFile.Write([]byte(privateKey)); err != nil {
		cleanup()
		return "", nil, fmt.Errorf("failed to write SSH private key to temporary file: %w", err)
	}
	if err := tmpFile.Close(); err != nil {
		cleanup()
		return "", nil, fmt.Errorf("failed to close temporary file: %w", err)
	}

	return keyFilePath, cleanup, nil
}

func selectSSHUser(needsRoot bool) string {
	if needsRoot {
		return "root"
	}
	return "ve"
}

func buildSSHFSCommand(ctx context.Context, keyFilePath, sshUser, localMountPoint string, port int) *exec.Cmd {
	return exec.CommandContext(ctx, // #nosec G204 -- keyFilePath is a securely created temp file, localMountPoint is user-provided
		"sshfs",
		"-o", fmt.Sprintf("IdentityFile=%s", keyFilePath),
		"-o", "StrictHostKeyChecking=no",
		"-o", "UserKnownHostsFile=/dev/null",
		"-o", "nomap=ignore",
		fmt.Sprintf("%s@localhost:/volume", sshUser),
		localMountPoint,
		"-p", fmt.Sprintf("%d", port),
	)
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

	container := buildContainer(publicKey, role, sshPort, needsRoot, image, cpuLimit)
	labels := buildPodLabels(pvcName, port, originalPodName)
	imagePullSecrets := buildImagePullSecrets(imageSecret)
	podSecurityContext := buildPodSecurityContext(needsRoot)

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

	if role != "proxy" {
		attachPVCToPod(pod, pvcName)
	}

	return pod
}

func buildContainer(publicKey, role string, sshPort int, needsRoot bool, image, cpuLimit string) corev1.Container {
	envVars := buildEnvVars(publicKey, role, sshPort, needsRoot)
	imageToUse := selectImage(image, needsRoot)
	resources := buildResourceRequirements(cpuLimit)

	return corev1.Container{
		Name:            "volume-exposer",
		Image:           imageToUse,
		ImagePullPolicy: corev1.PullAlways,
		Ports: []corev1.ContainerPort{
			{ContainerPort: int32(sshPort)}, // #nosec G115 -- sshPort is validated to be within valid port range (1024-65535)
		},
		Env:             envVars,
		SecurityContext: getSecurityContext(needsRoot),
		Resources:       resources,
	}
}

func buildEnvVars(publicKey, role string, sshPort int, needsRoot bool) []corev1.EnvVar {
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
	return envVars
}

func selectImage(image string, needsRoot bool) string {
	if image != "" {
		return image
	}
	if needsRoot {
		return PrivilegedImage
	}
	return Image
}

func buildResourceRequirements(cpuLimit string) corev1.ResourceRequirements {
	resources := corev1.ResourceRequirements{
		Requests: corev1.ResourceList{
			corev1.ResourceCPU:              resource.MustParse(CPURequest),
			corev1.ResourceMemory:           resource.MustParse(MemoryRequest),
			corev1.ResourceEphemeralStorage: resource.MustParse(EphemeralStorageRequest),
		},
		Limits: corev1.ResourceList{
			corev1.ResourceMemory:           resource.MustParse(MemoryLimit),
			corev1.ResourceEphemeralStorage: resource.MustParse(EphemeralStorageLimit),
		},
	}
	if cpuLimit != "" {
		resources.Limits[corev1.ResourceCPU] = resource.MustParse(cpuLimit)
	}
	return resources
}

func buildPodLabels(pvcName string, port int, originalPodName string) map[string]string {
	labels := map[string]string{
		"app":        "volume-exposer",
		"pvcName":    pvcName,
		"portNumber": fmt.Sprintf("%d", port),
	}
	if originalPodName != "" {
		labels["originalPodName"] = originalPodName
	}
	return labels
}

func buildImagePullSecrets(imageSecret string) []corev1.LocalObjectReference {
	if imageSecret == "" {
		return []corev1.LocalObjectReference{}
	}
	return []corev1.LocalObjectReference{{Name: imageSecret}}
}

func buildPodSecurityContext(needsRoot bool) *corev1.PodSecurityContext {
	runAsNonRoot := !needsRoot
	runAsUser := DefaultUserGroup
	runAsGroup := DefaultUserGroup
	if needsRoot {
		runAsUser = 0
		runAsGroup = 0
	}
	return &corev1.PodSecurityContext{
		RunAsNonRoot: &runAsNonRoot,
		RunAsUser:    &runAsUser,
		RunAsGroup:   &runAsGroup,
	}
}

func attachPVCToPod(pod *corev1.Pod, pvcName string) {
	pod.Spec.Containers[0].VolumeMounts = []corev1.VolumeMount{
		{MountPath: "/volume", Name: "my-pvc"},
	}
	pod.Spec.Volumes = []corev1.Volume{
		{
			Name: "my-pvc",
			VolumeSource: corev1.VolumeSource{
				PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
					ClaimName: pvcName,
				},
			},
		},
	}
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
