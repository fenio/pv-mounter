package plugin

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"runtime"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
)

func Clean(namespace, pvcName, localMountPoint string) error {
	// Unmount the local mount point
	var umountCmd *exec.Cmd
	if runtime.GOOS == "darwin" {
		umountCmd = exec.Command("umount", localMountPoint)
	} else {
		umountCmd = exec.Command("fusermount", "-u", localMountPoint)
	}
	umountCmd.Stdout = os.Stdout
	umountCmd.Stderr = os.Stderr
	if err := umountCmd.Run(); err != nil {
		return fmt.Errorf("failed to unmount SSHFS: %v", err)
	}
	fmt.Printf("Unmounted %s successfully\n", localMountPoint)

	// Build Kubernetes client
	clientset, err := BuildKubeClient()
	if err != nil {
		return err
	}

	ctx := context.TODO()
	// List the pod with the PVC name label
	podClient := clientset.CoreV1().Pods(namespace)
	podList, err := podClient.List(ctx, metav1.ListOptions{
		LabelSelector: fmt.Sprintf("pvcName=%s", pvcName),
	})
	if err != nil {
		return fmt.Errorf("failed to list pods: %v", err)
	}

	if len(podList.Items) == 0 {
		return fmt.Errorf("no pod found with PVC name label %s", pvcName)
	}

	podName := podList.Items[0].Name
	port := podList.Items[0].Labels["portNumber"]

	// Kill the port-forward process
	pkillCmd := exec.Command("pkill", "-f", fmt.Sprintf("kubectl port-forward pod/%s %s:2137", podName, port))
	pkillCmd.Stdout = os.Stdout
	pkillCmd.Stderr = os.Stderr
	if err := pkillCmd.Run(); err != nil {
		return fmt.Errorf("failed to kill port-forward process: %v", err)
	}
	fmt.Printf("Port-forward process for pod %s killed successfully\n", podName)

	// Remove the ephemeral container
	err = removeEphemeralContainer(clientset, namespace, podName)
	if err != nil {
		return fmt.Errorf("failed to remove ephemeral container: %v", err)
	}
	fmt.Printf("Ephemeral container in pod %s removed successfully\n", podName)

	// Delete the pod
	err = podClient.Delete(ctx, podName, metav1.DeleteOptions{})
	if err != nil {
		return fmt.Errorf("failed to delete pod: %v", err)
	}
	fmt.Printf("Pod %s deleted successfully\n", podName)

	return nil
}

func removeEphemeralContainer(clientset *kubernetes.Clientset, namespace, podName string) error {
	patchData, err := json.Marshal(map[string]interface{}{
		"spec": map[string]interface{}{
			"ephemeralContainers": []interface{}{},
		},
	})
	if err != nil {
		return fmt.Errorf("failed to marshal patch data: %v", err)
	}

	_, err = clientset.CoreV1().Pods(namespace).Patch(context.TODO(), podName, types.StrategicMergePatchType, patchData, metav1.PatchOptions{}, "ephemeralcontainers")
	if err != nil {
		return fmt.Errorf("failed to patch pod to remove ephemeral container: %v", err)
	}

	return nil
}
