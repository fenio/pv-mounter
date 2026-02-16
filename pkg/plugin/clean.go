// Package plugin implements the core functionality for mounting and cleaning PVCs.
package plugin

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"runtime"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// Clean unmounts a PVC and removes associated resources.
func Clean(ctx context.Context, namespace, pvcName, localMountPoint string) error {
	if err := ValidateKubernetesName(namespace, "namespace"); err != nil {
		return err
	}
	if err := ValidateKubernetesName(pvcName, "pvc-name"); err != nil {
		return err
	}

	var umountCmd *exec.Cmd
	if runtime.GOOS == "darwin" {
		umountCmd = exec.CommandContext(ctx, "umount", localMountPoint)
	} else {
		umountCmd = exec.CommandContext(ctx, "fusermount", "-u", localMountPoint)
	}
	umountCmd.Stdout = os.Stdout
	umountCmd.Stderr = os.Stderr
	if err := umountCmd.Run(); err != nil {
		return fmt.Errorf("failed to unmount SSHFS: %w", err)
	}
	fmt.Printf("Unmounted %s successfully\n", localMountPoint)

	clientset, err := BuildKubeClient()
	if err != nil {
		return err
	}

	// Look for a standalone pod with the PVC name label (RWX case)
	podClient := clientset.CoreV1().Pods(namespace)
	podList, err := podClient.List(ctx, metav1.ListOptions{
		LabelSelector: fmt.Sprintf("pvcName=%s,app=volume-exposer", pvcName),
	})
	if err != nil {
		return fmt.Errorf("failed to list pods: %w", err)
	}

	if len(podList.Items) > 0 {
		// RWX/standalone case: kill port-forward and delete standalone pod
		podName := podList.Items[0].Name

		pkillCmd := exec.CommandContext(ctx, "pkill", "-f", fmt.Sprintf("kubectl port-forward pod/%s", podName)) // #nosec G204 -- podName is from validated Kubernetes resources
		pkillCmd.Stdout = os.Stdout
		pkillCmd.Stderr = os.Stderr
		if err := pkillCmd.Run(); err != nil {
			return fmt.Errorf("failed to kill port-forward process: %w", err)
		}
		fmt.Printf("Port-forward process for pod %s killed successfully\n", podName)

		err = podClient.Delete(ctx, podName, metav1.DeleteOptions{})
		if err != nil {
			return fmt.Errorf("failed to delete pod: %w", err)
		}
		fmt.Printf("Pod %s deleted successfully\n", podName)

		return nil
	}

	// RWO case: find the workload pod using the PVC and clean up ephemeral container
	return cleanRWO(ctx, clientset, namespace, pvcName)
}

// cleanRWO handles cleanup for RWO volumes mounted via ephemeral containers.
func cleanRWO(ctx context.Context, clientset *kubernetes.Clientset, namespace, pvcName string) error {
	podList, err := clientset.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("failed to list pods: %w", err)
	}

	podName := findPodUsingPVC(podList.Items, pvcName)
	if podName == "" {
		return fmt.Errorf("no pod found using PVC %s", pvcName)
	}

	pkillCmd := exec.CommandContext(ctx, "pkill", "-f", fmt.Sprintf("kubectl port-forward pod/%s", podName)) // #nosec G204 -- podName is from validated Kubernetes resources
	pkillCmd.Stdout = os.Stdout
	pkillCmd.Stderr = os.Stderr
	if err := pkillCmd.Run(); err != nil {
		return fmt.Errorf("failed to kill port-forward process: %w", err)
	}
	fmt.Printf("Port-forward process for pod %s killed successfully\n", podName)

	err = killProcessInEphemeralContainer(ctx, clientset, namespace, podName)
	if err != nil {
		return fmt.Errorf("failed to kill process in ephemeral container: %w", err)
	}
	fmt.Printf("Process in ephemeral container killed successfully in pod %s\n", podName)

	return nil
}

func killProcessInEphemeralContainer(ctx context.Context, clientset kubernetes.Interface, namespace, podName string) error {
	existingPod, err := clientset.CoreV1().Pods(namespace).Get(ctx, podName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get existing pod: %w", err)
	}

	if len(existingPod.Spec.EphemeralContainers) == 0 {
		return fmt.Errorf("no ephemeral containers found in pod %s", podName)
	}

	ephemeralContainerName := existingPod.Spec.EphemeralContainers[0].Name
	fmt.Printf("Ephemeral container name is %s\n", ephemeralContainerName)

	killCmd := []string{"pkill", "sshd"}

	cmd := exec.CommandContext(ctx, "kubectl", append([]string{"exec", podName, "-n", namespace, "-c", ephemeralContainerName, "--"}, killCmd...)...) // #nosec G204 -- podName, namespace, and ephemeralContainerName are from validated Kubernetes resources
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to kill process in container %s of pod %s: %w", ephemeralContainerName, podName, err)
	}
	return nil
}
