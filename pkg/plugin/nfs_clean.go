package plugin

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// cleanNFS handles cleanup for NFS-mounted PVCs.
func cleanNFS(ctx context.Context, namespace, pvcName, localMountPoint string) error {
	// NFS always uses umount
	umountCmd := exec.CommandContext(ctx, "umount", localMountPoint) // #nosec G204 -- localMountPoint is user-provided
	umountCmd.Stdout = os.Stdout
	umountCmd.Stderr = os.Stderr
	if err := umountCmd.Run(); err != nil {
		return fmt.Errorf("failed to unmount NFS: %w", err)
	}
	fmt.Printf("Unmounted %s successfully\n", localMountPoint)

	clientset, err := BuildKubeClient()
	if err != nil {
		return err
	}

	// Look for a standalone NFS pod with the PVC name and backend=nfs label (RWX case)
	podClient := clientset.CoreV1().Pods(namespace)
	podList, err := podClient.List(ctx, metav1.ListOptions{
		LabelSelector: fmt.Sprintf("pvcName=%s,app=volume-exposer,backend=nfs", pvcName),
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

	// RWO case: find the workload pod using the PVC and clean up NFS ephemeral container
	return cleanNFSRWO(ctx, clientset, namespace, pvcName)
}

// cleanNFSRWO handles cleanup for RWO volumes mounted via NFS ephemeral containers.
func cleanNFSRWO(ctx context.Context, clientset *kubernetes.Clientset, namespace, pvcName string) error {
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

	err = killNFSProcessInEphemeralContainer(ctx, clientset, namespace, podName)
	if err != nil {
		return fmt.Errorf("failed to kill NFS process in ephemeral container: %w", err)
	}
	fmt.Printf("NFS process in ephemeral container killed successfully in pod %s\n", podName)

	return nil
}

// killNFSProcessInEphemeralContainer kills the ganesha.nfsd process in an NFS ephemeral container.
func killNFSProcessInEphemeralContainer(ctx context.Context, clientset kubernetes.Interface, namespace, podName string) error {
	existingPod, err := clientset.CoreV1().Pods(namespace).Get(ctx, podName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get existing pod: %w", err)
	}

	// Find the NFS ephemeral container (name prefix nfs-ganesha-ephemeral-)
	ephemeralContainerName := ""
	for _, ec := range existingPod.Spec.EphemeralContainers {
		if strings.HasPrefix(ec.Name, "nfs-ganesha-ephemeral-") {
			ephemeralContainerName = ec.Name
		}
	}
	if ephemeralContainerName == "" {
		return fmt.Errorf("no NFS ephemeral container found in pod %s", podName)
	}

	fmt.Printf("NFS ephemeral container name is %s\n", ephemeralContainerName)

	killCmd := []string{"pkill", "ganesha.nfsd"}

	cmd := exec.CommandContext(ctx, "kubectl", append([]string{"exec", podName, "-n", namespace, "-c", ephemeralContainerName, "--"}, killCmd...)...) // #nosec G204 -- podName, namespace, and ephemeralContainerName are from validated Kubernetes resources
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to kill NFS process in container %s of pod %s: %w", ephemeralContainerName, podName, err)
	}
	return nil
}
