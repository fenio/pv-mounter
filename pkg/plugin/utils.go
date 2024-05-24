package plugin

import (
	"fmt"
	"os"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

// BuildKubeClient creates a Kubernetes client from a kubeconfig
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
