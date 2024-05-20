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
	"k8s.io/client-go/tools/clientcmd"
)

// BuildKubeClient creates a Kubernetes client
func BuildKubeClient() (*kubernetes.Clientset, error) {
	kubeconfig := os.Getenv("KUBECONFIG")
	if kubeconfig == "" {
		home := os.Getenv("HOME")
		kubeconfig = fmt.Sprintf("%s/.kube/config", home)
	}

	config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		return nil, fmt.Errorf("failed to build Kubernetes config: %v", err)
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create Kubernetes client: %v", err)
	}

	return clientset, nil
}

// Mount mounts a PVC to a local directory
func Mount(namespace, pvcName, localMountPoint string) error {
	if _, err := os.Stat(localMountPoint); os.IsNotExist(err) {
		return fmt.Errorf("local mount point %s does not exist", localMountPoint)
	}

	rand.Seed(time.Now().UnixNano())
	suffix := randSeq(5)
	podName := fmt.Sprintf("volume-exposer-%s", suffix)
	port := rand.Intn(64511) + 1024 // Generate a random port between 1024 and 65535

	sshKeyPath := fmt.Sprintf("%s/.ssh/id_rsa.pub", os.Getenv("HOME"))
	sshKey, err := ioutil.ReadFile(sshKeyPath)
	if err != nil {
		return fmt.Errorf("failed to read SSH public key: %v", err)
	}

	clientset, err := BuildKubeClient()
	if err != nil {
		return err
	}

	ctx := context.TODO()
	pvcClient := clientset.CoreV1().PersistentVolumeClaims(namespace)
	pvc, err := pvcClient.Get(ctx, pvcName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get PVC: %v", err)
	}

	if pvc.Status.Phase == corev1.ClaimBound {
		pvName := pvc.Spec.VolumeName
		pvClient := clientset.CoreV1().PersistentVolumes()
		pv, err := pvClient.Get(ctx, pvName, metav1.GetOptions{})
		if err != nil {
			return fmt.Errorf("failed to get PV: %v", err)
		}

		accessModes := pv.Spec.AccessModes
		for _, mode := range accessModes {
			if mode == corev1.ReadWriteOnce {
				pods, err := clientset.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{})
				if err != nil {
					return fmt.Errorf("failed to list pods: %v", err)
				}

				for _, pod := range pods.Items {
					for _, volume := range pod.Spec.Volumes {
						if volume.PersistentVolumeClaim != nil && volume.PersistentVolumeClaim.ClaimName == pvcName {
							return fmt.Errorf("PVC %s is already in use by pod %s and cannot be mounted because it has RWO access mode", pvcName, pod.Name)
						}
					}
				}
			}
		}
	}

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: podName,
			Labels: map[string]string{
				"app":        "volume-exposer",
				"pvcName":    pvcName,
				"portNumber": fmt.Sprintf("%d", port),
			},
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:  "volume-exposer",
					Image: "bfenski/volume-exposer:latest",
					Ports: []corev1.ContainerPort{
						{
							ContainerPort: 22,
						},
					},
					VolumeMounts: []corev1.VolumeMount{
						{
							MountPath: "/volume",
							Name:      "my-pvc",
						},
					},
					Env: []corev1.EnvVar{
						{
							Name:  "SSH_KEY",
							Value: string(sshKey),
						},
					},
				},
			},
			Volumes: []corev1.Volume{
				{
					Name: "my-pvc",
					VolumeSource: corev1.VolumeSource{
						PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
							ClaimName: pvcName,
						},
					},
				},
			},
		},
	}

	podClient := clientset.CoreV1().Pods(namespace)

	createdPod, err := podClient.Create(ctx, pod, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("failed to create pod: %v", err)
	}

	fmt.Printf("Pod %s created successfully\n", createdPod.Name)

	err = wait.PollImmediate(time.Second, 5*time.Minute, func() (bool, error) {
		pod, err := podClient.Get(ctx, podName, metav1.GetOptions{})
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
	if err != nil {
		return fmt.Errorf("failed to wait for pod to be ready: %v", err)
	}

	portForwardCmd := exec.Command("kubectl", "port-forward", fmt.Sprintf("pod/%s", podName), fmt.Sprintf("%d:22", port), "-n", namespace)
	portForwardCmd.Stdout = os.Stdout
	portForwardCmd.Stderr = os.Stderr
	if err := portForwardCmd.Start(); err != nil {
		return fmt.Errorf("failed to start port-forward: %v", err)
	}

	time.Sleep(5 * time.Second)

	sshfsCmd := exec.Command("sshfs", "-o", "StrictHostKeyChecking=no,UserKnownHostsFile=/dev/null", fmt.Sprintf("root@localhost:/volume"), localMountPoint, "-p", fmt.Sprintf("%d", port))
	sshfsCmd.Stdout = os.Stdout
	sshfsCmd.Stderr = os.Stderr
	sshfsCmd.Stdin = os.Stdin
	if err := sshfsCmd.Run(); err != nil {
		return fmt.Errorf("failed to mount PVC using SSHFS: %v", err)
	}

	fmt.Printf("PVC %s mounted successfully to %s\n", pvcName, localMountPoint)

	return nil
}

func randSeq(n int) string {
	letters := []rune("abcdefghijklmnopqrstuvwxyz0123456789")
	b := make([]rune, n)
	for i := range b {
		b[i] = letters[rand.Intn(len(letters))]
	}
	return string(b)
}
