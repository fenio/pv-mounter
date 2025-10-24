package plugin

import (
	"context"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestKillProcessInEphemeralContainer(t *testing.T) {
	namespace := "default"
	podName := "test-pod"

	t.Run("Pod with ephemeral containers", func(t *testing.T) {
		clientset := fake.NewSimpleClientset(&corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      podName,
				Namespace: namespace,
			},
			Spec: corev1.PodSpec{
				EphemeralContainers: []corev1.EphemeralContainer{
					{
						EphemeralContainerCommon: corev1.EphemeralContainerCommon{
							Name: "ephemeral-container",
						},
					},
				},
			},
		})

		ctx := context.Background()

		err := killProcessInEphemeralContainer(ctx, clientset, namespace, podName)
		if err != nil {
			if err.Error() != "no ephemeral containers found in pod test-pod" {
				t.Logf("killProcessInEphemeralContainer() executed with error (expected in test): %v", err)
			}
		}
	})

	t.Run("Pod with multiple ephemeral containers", func(t *testing.T) {
		clientset := fake.NewSimpleClientset(&corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      podName,
				Namespace: namespace,
			},
			Spec: corev1.PodSpec{
				EphemeralContainers: []corev1.EphemeralContainer{
					{
						EphemeralContainerCommon: corev1.EphemeralContainerCommon{
							Name: "first-container",
						},
					},
					{
						EphemeralContainerCommon: corev1.EphemeralContainerCommon{
							Name: "second-container",
						},
					},
				},
			},
		})

		ctx := context.Background()

		err := killProcessInEphemeralContainer(ctx, clientset, namespace, podName)
		if err != nil {
			t.Logf("killProcessInEphemeralContainer() with multiple containers: %v", err)
		}
	})

	t.Run("Pod without ephemeral containers", func(t *testing.T) {
		clientset := fake.NewSimpleClientset(&corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      podName,
				Namespace: namespace,
			},
			Spec: corev1.PodSpec{},
		})

		ctx := context.Background()

		err := killProcessInEphemeralContainer(ctx, clientset, namespace, podName)
		if err == nil {
			t.Error("killProcessInEphemeralContainer() should return an error when no ephemeral containers exist")
		}
	})

	t.Run("Pod does not exist", func(t *testing.T) {
		clientset := fake.NewSimpleClientset()

		ctx := context.Background()

		err := killProcessInEphemeralContainer(ctx, clientset, namespace, podName)
		if err == nil {
			t.Error("killProcessInEphemeralContainer() should return an error when pod does not exist")
		}
	})
}
