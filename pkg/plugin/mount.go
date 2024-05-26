package plugin

import (
	"context"
	"fmt"
	"io/ioutil"
	"math/rand"
	"os"
	"os/exec"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
)

func Mount(namespace, pvcName, localMountPoint string) error {
	checkSSHFS()

	if _, err := os.Stat(localMountPoint); os.IsNotExist(err) {
		return fmt.Errorf("local mount point %s does not exist", localMountPoint)
	}

	clientset, err := BuildKubeClient()
	if err != nil {
		return err
	}

	pvc, err := checkPVCUsage(clientset, namespace, pvcName)
	if err != nil {
		return err
	}

	canMount, podUsingPVC, err := checkPVAccessMode(clientset, pvc, namespace) // Corrected the number of parameters
	if err != nil {
		return err
	}

	if !canMount {
		fmt.Printf("RWO volume is currently mounted by another pod: %s; mounting in this mode is not implemented yet.\n", podUsingPVC)
		return fmt.Errorf("mount operation for RWO volume that is already in use by pod %s is not implemented yet", podUsingPVC)
	}

	sshKey, err := readSSHKey()
	if err != nil {
		return err
	}

	privateKey, publicKey, err := GenerateKeyPair(2048)
	if err != nil {
		fmt.Printf("Error generating key pair: %v\n", err)
		return err
	}

	_ = privateKey
	_ = publicKey

	podName, port, err := setupPod(clientset, namespace, pvcName, sshKey, "standalone")
	if err != nil {
		return err
	}

	if err := waitForPodReady(clientset, namespace, podName); err != nil {
		return err
	}

	if err := setupPortForwarding(namespace, podName, port); err != nil {
		return err
	}

	return mountPVCOverSSH(namespace, podName, port, localMountPoint, pvcName)
}

func checkPVAccessMode(clientset *kubernetes.Clientset, pvc *corev1.PersistentVolumeClaim, namespace string) (bool, string, error) {
	pvName := pvc.Spec.VolumeName
	pv, err := clientset.CoreV1().PersistentVolumes().Get(context.TODO(), pvName, metav1.GetOptions{})
	if err != nil {
		return true, "", fmt.Errorf("failed to get PV: %v", err)
	}

	// Assuming pv is now being checked for its AccessModes.
	if contains(pv.Spec.AccessModes, corev1.ReadWriteOnce) {
		podList, err := clientset.CoreV1().Pods(namespace).List(context.TODO(), metav1.ListOptions{})
		if err != nil {
			return true, "", fmt.Errorf("failed to list pods: %v", err)
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

func readSSHKey() (string, error) {
	sshKeyPath := fmt.Sprintf("%s/.ssh/id_rsa.pub", os.Getenv("HOME"))
	sshKey, err := ioutil.ReadFile(sshKeyPath)
	if err != nil {
		return "", fmt.Errorf("failed to read SSH public key: %v", err)
	}
	return string(sshKey), nil
}

func checkPVCUsage(clientset *kubernetes.Clientset, namespace, pvcName string) (*corev1.PersistentVolumeClaim, error) {
	pvc, err := clientset.CoreV1().PersistentVolumeClaims(namespace).Get(context.TODO(), pvcName, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get PVC: %v", err)
	}
	if pvc.Status.Phase != corev1.ClaimBound {
		return nil, fmt.Errorf("PVC %s is not bound", pvcName)
	}
	return pvc, nil
}

func setupPod(clientset *kubernetes.Clientset, namespace, pvcName, sshKey, role string) (string, int, error) {
	podName, port := generatePodNameAndPort(pvcName, role)
	pod := createPodSpec(podName, port, pvcName, sshKey, role)
	if _, err := clientset.CoreV1().Pods(namespace).Create(context.TODO(), pod, metav1.CreateOptions{}); err != nil {
		return "", 0, fmt.Errorf("failed to create pod: %v", err)
	}
	fmt.Printf("Pod %s created successfully\n", podName)
	return podName, port, nil
}

func waitForPodReady(clientset *kubernetes.Clientset, namespace, podName string) error {
	return wait.PollImmediate(time.Second, 5*time.Minute, func() (bool, error) {
		pod, err := clientset.CoreV1().Pods(namespace).Get(context.TODO(), podName, metav1.GetOptions{})
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

func setupPortForwarding(namespace, podName string, port int) error {
	cmd := exec.Command("kubectl", "port-forward", fmt.Sprintf("pod/%s", podName), fmt.Sprintf("%d:2137", port), "-n", namespace)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start port-forward: %v", err)
	}
	time.Sleep(5 * time.Second) // Wait a bit for the port forwarding to establish
	return nil
}

func mountPVCOverSSH(namespace, podName string, port int, localMountPoint, pvcName string) error {
	sshfsCmd := exec.Command("sshfs", "-o", "StrictHostKeyChecking=no,UserKnownHostsFile=/dev/null", fmt.Sprintf("ve@localhost:/volume"), localMountPoint, "-p", fmt.Sprintf("%d", port))
	sshfsCmd.Stdout = os.Stdout
	sshfsCmd.Stderr = os.Stderr
	if err := sshfsCmd.Run(); err != nil {
		return fmt.Errorf("failed to mount PVC using SSHFS: %v", err)
	}
	fmt.Printf("PVC %s mounted successfully to %s\n", pvcName, localMountPoint)
	return nil
}

func generatePodNameAndPort(pvcName, role string) (string, int) {
	rand.Seed(time.Now().UnixNano())
	suffix := randSeq(5)
	baseName := "volume-exposer"
	if role == "proxy" {
		baseName = "volume-exposer-proxy"
	}
	podName := fmt.Sprintf("%s-%s", baseName, suffix)
	port := rand.Intn(64511) + 1024 // Generate a random port between 1024 and 65535
	return podName, port
}

func createPodSpec(podName string, port int, pvcName, sshKey, role string) *corev1.Pod {
	envVars := []corev1.EnvVar{
		{
			Name:  "SSH_KEY",
			Value: sshKey,
		},
	}

	// Add the ROLE environment variable if the role is "standalone" or "proxy"
	if role == "standalone" || role == "proxy" {
		envVars = append(envVars, corev1.EnvVar{
			Name:  "ROLE",
			Value: role,
		})
	}

	runAsNonRoot := true
	runAsUser := int64(2137)
	runAsGroup := int64(2137)
	allowPrivilegeEscalation := false
	readOnlyRootFilesystem := false

	container := corev1.Container{
		Name:  "volume-exposer",
		Image: "bfenski/volume-exposer:latest",
		Ports: []corev1.ContainerPort{
			{
				ContainerPort: 2137,
			},
		},
		Env: envVars,
		SecurityContext: &corev1.SecurityContext{
			AllowPrivilegeEscalation: &allowPrivilegeEscalation,
			ReadOnlyRootFilesystem:   &readOnlyRootFilesystem,
			Capabilities: &corev1.Capabilities{
				Drop: []corev1.Capability{"ALL"},
			},
		},
	}

	podSpec := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: podName,
			Labels: map[string]string{
				"app":        "volume-exposer",
				"pvcName":    pvcName,
				"portNumber": fmt.Sprintf("%d", port),
			},
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{container},
			SecurityContext: &corev1.PodSecurityContext{
				RunAsNonRoot: &runAsNonRoot,
				RunAsUser:    &runAsUser,
				RunAsGroup:   &runAsGroup,
			},
		},
	}

	// Only mount the volume if the role is not "proxy"
	if role != "proxy" {
		container.VolumeMounts = []corev1.VolumeMount{
			{
				MountPath: "/volume",
				Name:      "my-pvc",
			},
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
		// Update the container in the podSpec with the volume mounts
		podSpec.Spec.Containers[0] = container
	}

	return podSpec
}
