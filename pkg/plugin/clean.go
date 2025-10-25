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

// Clean unmounts a PVC and removes associated resources including proxy pods and port-forwarding.
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

	// Build Kubernetes client
	clientset, err := BuildKubeClient()
	if err != nil {
		return err
	}

	// List the pod with the PVC name label
	podClient := clientset.CoreV1().Pods(namespace)
	podList, err := podClient.List(ctx, metav1.ListOptions{
		LabelSelector: fmt.Sprintf("pvcName=%s", pvcName),
	})
	if err != nil {
		return fmt.Errorf("failed to list pods: %w", err)
	}

	if len(podList.Items) == 0 {
		return fmt.Errorf("no pod found with PVC name label %s", pvcName)
	}

	podName := podList.Items[0].Name
	port := podList.Items[0].Labels["portNumber"]

	pkillCmd := exec.CommandContext(ctx, "pkill", "-f", fmt.Sprintf("kubectl port-forward pod/%s %s:2137", podName, port)) // #nosec G204 -- podName and port are from validated Kubernetes resources
	pkillCmd.Stdout = os.Stdout
	pkillCmd.Stderr = os.Stderr
	if err := pkillCmd.Run(); err != nil {
		return fmt.Errorf("failed to kill port-forward process: %w", err)
	}
	fmt.Printf("Port-forward process for pod %s killed successfully\n", podName)

	// Check for original pod
	originalPodName := podList.Items[0].Labels["originalPodName"]
	if originalPodName != "" {
		err = killProcessInEphemeralContainer(ctx, clientset, namespace, originalPodName)
		if err != nil {
			return fmt.Errorf("failed to kill process in ephemeral container: %w", err)
		}
		fmt.Printf("Process in ephemeral container killed successfully in pod %s\n", originalPodName)
	}

	// Delete the proxy pod
	err = podClient.Delete(ctx, podName, metav1.DeleteOptions{})
	if err != nil {
		return fmt.Errorf("failed to delete pod: %w", err)
	}
	fmt.Printf("Proxy pod %s deleted successfully\n", podName)

	return nil
}

func killProcessInEphemeralContainer(ctx context.Context, clientset kubernetes.Interface, namespace, podName string) error {
	// Retrieve the existing pod to get the ephemeral container name
	existingPod, err := clientset.CoreV1().Pods(namespace).Get(ctx, podName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get existing pod: %w", err)
	}

	if len(existingPod.Spec.EphemeralContainers) == 0 {
		return fmt.Errorf("no ephemeral containers found in pod %s", podName)
	}

	ephemeralContainerName := existingPod.Spec.EphemeralContainers[0].Name
	fmt.Printf("Ephemeral container name is %s\n", ephemeralContainerName)

	// Command to kill the process (adjust the process name or ID as necessary)
	killCmd := []string{"pkill", "-f", "tail"} // Replace "tail" with the actual process name or use a specific PID

	cmd := exec.CommandContext(ctx, "kubectl", append([]string{"exec", podName, "-n", namespace, "-c", ephemeralContainerName, "--"}, killCmd...)...) // #nosec G204 -- podName, namespace, and ephemeralContainerName are from validated Kubernetes resources
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to kill process in container %s of pod %s: %w", ephemeralContainerName, podName, err)
	}
	return nil
}
