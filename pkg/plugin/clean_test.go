package plugin

import (
	"context"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
)

func TestClean(t *testing.T) {
	// Create a fake clientset
	clientset := fake.NewSimpleClientset()

	// Create a test pod
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pod",
			Namespace: "default",
			Labels: map[string]string{
				"pvcName":    "test-pvc",
				"portNumber": "12345",
			},
		},
	}
	_, err := clientset.CoreV1().Pods("default").Create(context.TODO(), pod, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("Error creating test pod: %v", err)
	}

	// Mock BuildKubeClient to return our fake clientset
	oldBuildKubeClient := BuildKubeClient
	BuildKubeClient = func() (*kubernetes.Clientset, error) {
		return clientset, nil
	}
	defer func() { BuildKubeClient = oldBuildKubeClient }()

	// Call the Clean function
	err = Clean(context.TODO(), "default", "test-pvc", "/tmp/test-mount")
	if err != nil {
		t.Errorf("Clean function returned an error: %v", err)
	}

	// Verify that the pod was deleted
	_, err = clientset.CoreV1().Pods("default").Get(context.TODO(), "test-pod", metav1.GetOptions{})
	if err == nil {
		t.Error("Pod was not deleted")
	}
}

func TestKillProcessInEphemeralContainer(t *testing.T) {
	// Create a fake clientset
	clientset := fake.NewSimpleClientset()

	// Create a test pod with an ephemeral container
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pod",
			Namespace: "default",
		},
		Spec: corev1.PodSpec{
			EphemeralContainers: []corev1.EphemeralContainer{
				{
					EphemeralContainerCommon: corev1.EphemeralContainerCommon{
						Name: "test-ephemeral",
					},
				},
			},
		},
	}
	_, err := clientset.CoreV1().Pods("default").Create(context.TODO(), pod, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("Error creating test pod: %v", err)
	}

	// Call the killProcessInEphemeralContainer function
	err = killProcessInEphemeralContainer(context.TODO(), clientset, "default", "test-pod")
	if err != nil {
		t.Errorf("killProcessInEphemeralContainer function returned an error: %v", err)
	}

	// Note: We can't easily verify if the process was killed in a unit test,
	// but we can at least check that the function runs without error
}
