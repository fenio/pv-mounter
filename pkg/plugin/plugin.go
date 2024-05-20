package plugin

import (
	"context"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/cli-runtime/pkg/genericclioptions"
)

// RunPlugin runs the plugin
func RunPlugin(configFlags *genericclioptions.ConfigFlags, namespaceName chan<- string) error {
	// Build Kubernetes client
	clientset, err := BuildKubeClient()
	if err != nil {
		return err
	}

	ctx := context.TODO()
	// List namespaces
	namespaces, err := clientset.CoreV1().Namespaces().List(ctx, metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("failed to list namespaces: %v", err)
	}

	for _, ns := range namespaces.Items {
		namespaceName <- ns.Name
	}

	return nil
}
