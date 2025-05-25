package plugin

import (
	"context"
	"encoding/json"
	"errors" // Added for errors.Is
	"fmt"
	"os"
	"strings"
	"testing"
	"time" // Added for time.Millisecond

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types" // Ensured import for types.StrategicMergePatchType
	// "k8s.io/apimachinery/pkg/util/wait" // Removed as it's not directly used by tests
	"k8s.io/client-go/kubernetes" // For kubernetes.Interface
	"k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"
)

func TestValidateMountPoint(t *testing.T) {
	t.Run("Mount point exists", func(t *testing.T) {
		tempDir := t.TempDir()
		err := validateMountPoint(tempDir)
		if err != nil {
			t.Errorf("validateMountPoint(%s) returned an unexpected error: %v", tempDir, err)
		}
	})

	t.Run("Mount point does not exist", func(t *testing.T) {
		nonExistentPath := "/path/that/does/not/exist"
		err := validateMountPoint(nonExistentPath)
		if err == nil {
			t.Errorf("validateMountPoint(%s) should have returned an error, but it did not", nonExistentPath)
		}
	})
}

func TestSetupPod(t *testing.T) {
	namespace := "default"
	pvcName := "test-pvc"
	publicKey := "test-public-key"
	role := "standalone"
	sshPort := DefaultSSHPort
	needsRoot := false
	image := "" // Use default image
	imageSecret := ""
	cpuLimit := ""

	t.Run("Successful pod creation", func(t *testing.T) {
		clientset := fake.NewSimpleClientset() // No objects needed for just Create
		ctx := context.Background()

		podName, port, err := setupPod(ctx, clientset, namespace, pvcName, publicKey, role, sshPort, "", needsRoot, image, imageSecret, cpuLimit)

		if err != nil {
			t.Errorf("setupPod() returned error: %v, want nil", err)
		}
		if podName == "" {
			t.Error("setupPod() returned empty podName, want non-empty")
		}
		if port == 0 {
			t.Error("setupPod() returned 0 for port, want non-zero")
		}

		// Verify that a pod was created
		actions := clientset.Actions()
		if len(actions) == 0 {
			t.Fatalf("setupPod() did not call any actions on the clientset")
		}
		createAction, ok := actions[0].(k8stesting.CreateAction)
		if !ok {
			t.Fatalf("Expected a CreateAction, got %T", actions[0])
		}
		if createAction.GetResource().Resource != "pods" {
			t.Errorf("Expected action on 'pods', got '%s'", createAction.GetResource().Resource)
		}
		// Pod name is generated, so we can check if it's part of the created object's name
		createdPod, ok := createAction.GetObject().(*corev1.Pod)
		if !ok {
			t.Fatalf("CreateAction did not create a *corev1.Pod")
		}
		// generatePodNameAndPort creates "volume-exposer-<suffix>" for standalone or "volume-exposer-proxy-<suffix>" for proxy
		expectedPrefix := "volume-exposer-"
		if role == "proxy" {
			expectedPrefix = "volume-exposer-proxy-"
		}
		if !strings.HasPrefix(createdPod.Name, expectedPrefix) {
			t.Errorf("Created pod name '%s' does not match expected prefix '%s'", createdPod.Name, expectedPrefix)
		}
		if createdPod.Name != podName {
			t.Errorf("Returned podName '%s' does not match created pod name '%s'", podName, createdPod.Name)
		}
	})

	t.Run("Failure when pod creation fails", func(t *testing.T) {
		expectedError := fmt.Errorf("simulated API error creating pod")
		clientset := fake.NewSimpleClientset()
		clientset.PrependReactor("create", "pods", func(action k8stesting.Action) (handled bool, ret runtime.Object, err error) {
			return true, nil, expectedError
		})
		ctx := context.Background()

		podName, port, err := setupPod(ctx, clientset, namespace, pvcName, publicKey, role, sshPort, "", needsRoot, image, imageSecret, cpuLimit)

		if err == nil {
			t.Errorf("setupPod() returned nil error, want '%v'", expectedError)
		} else if !strings.Contains(err.Error(), expectedError.Error()) {
			t.Errorf("setupPod() returned error: %v, want error containing '%s'", err, expectedError.Error())
		}
		if podName != "" {
			t.Errorf("setupPod() returned podName '%s', want empty string", podName)
		}
		if port != 0 {
			t.Errorf("setupPod() returned port %d, want 0", port)
		}
	})
}

func TestValidateMountPoint_FileInsteadOfDirectory(t *testing.T) {
	tempFile, err := os.CreateTemp("", "testfile")
	if err != nil {
		t.Fatalf("Failed to create temporary file: %v", err)
	}
	defer os.Remove(tempFile.Name())
	err = validateMountPoint(tempFile.Name())
	if err != nil {
		t.Errorf("validateMountPoint(%s) returned an unexpected error: %v", tempFile.Name(), err)
	}
}

func TestCheckPVCUsage(t *testing.T) {
	namespace := "default"
	pvcName := "test-pvc"

	t.Run("PVC is bound", func(t *testing.T) {
		pvc := &corev1.PersistentVolumeClaim{
			ObjectMeta: metav1.ObjectMeta{Name: pvcName, Namespace: namespace},
			Status:     corev1.PersistentVolumeClaimStatus{Phase: corev1.ClaimBound},
		}
		clientset := fake.NewSimpleClientset(pvc)
		ctx := context.Background()
		resultPVC, err := checkPVCUsage(ctx, clientset, namespace, pvcName)
		if err != nil {
			t.Errorf("checkPVCUsage() returned error: %v, want nil", err)
		}
		if resultPVC == nil || resultPVC.Name != pvcName {
			t.Errorf("checkPVCUsage() returned PVC name %v, want %s", resultPVC, pvcName)
		}
	})

	t.Run("PVC is not bound - Pending", func(t *testing.T) {
		pvc := &corev1.PersistentVolumeClaim{
			ObjectMeta: metav1.ObjectMeta{Name: pvcName, Namespace: namespace},
			Status:     corev1.PersistentVolumeClaimStatus{Phase: corev1.ClaimPending},
		}
		clientset := fake.NewSimpleClientset(pvc)
		ctx := context.Background()
		_, err := checkPVCUsage(ctx, clientset, namespace, pvcName)
		if err == nil {
			t.Errorf("checkPVCUsage() returned nil error, want error for unbound PVC (Pending)")
		}
	})

	t.Run("PVC is not bound - Lost", func(t *testing.T) {
		pvc := &corev1.PersistentVolumeClaim{
			ObjectMeta: metav1.ObjectMeta{Name: pvcName, Namespace: namespace},
			Status:     corev1.PersistentVolumeClaimStatus{Phase: corev1.ClaimLost},
		}
		clientset := fake.NewSimpleClientset(pvc)
		ctx := context.Background()
		_, err := checkPVCUsage(ctx, clientset, namespace, pvcName)
		if err == nil {
			t.Errorf("checkPVCUsage() returned nil error, want error for unbound PVC (Lost)")
		}
	})

	t.Run("Getting PVC fails - Not Found", func(t *testing.T) {
		clientset := fake.NewSimpleClientset()
		ctx := context.Background()
		_, err := checkPVCUsage(ctx, clientset, namespace, pvcName)
		if err == nil {
			t.Errorf("checkPVCUsage() returned nil error, want error for non-existent PVC")
		}
	})

	t.Run("Getting PVC fails - API error", func(t *testing.T) {
		clientset := fake.NewSimpleClientset()
		clientset.PrependReactor("get", "persistentvolumeclaims", func(action k8stesting.Action) (bool, runtime.Object, error) {
			return true, nil, fmt.Errorf("simulated API error")
		})
		ctx := context.Background()
		_, err := checkPVCUsage(ctx, clientset, namespace, pvcName)
		if err == nil {
			t.Errorf("checkPVCUsage() returned nil error, want error for API error")
		}
	})
}

func TestCheckPVAccessMode(t *testing.T) {
	namespace := "default"
	pvcName := "test-pvc"
	pvName := "test-pv"
	podName := "test-pod"

	t.Run("RWO access mode, no pod using PVC", func(t *testing.T) {
		pvc := &corev1.PersistentVolumeClaim{ObjectMeta: metav1.ObjectMeta{Name: pvcName, Namespace: namespace}, Spec: corev1.PersistentVolumeClaimSpec{VolumeName: pvName}}
		pv := &corev1.PersistentVolume{ObjectMeta: metav1.ObjectMeta{Name: pvName}, Spec: corev1.PersistentVolumeSpec{AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce}}}
		clientset := fake.NewSimpleClientset(pvc, pv)
		ctx := context.Background()
		canBeMounted, resultPodName, err := checkPVAccessMode(ctx, clientset, pvc, namespace)
		if err != nil {
			t.Errorf("checkPVAccessMode() error = %v, wantNil", err)
		}
		if !canBeMounted || resultPodName != "" {
			t.Errorf("checkPVAccessMode() = (%v, %s), want (true, \"\")", canBeMounted, resultPodName)
		}
	})

	t.Run("RWO access mode, pod using PVC", func(t *testing.T) {
		pvc := &corev1.PersistentVolumeClaim{ObjectMeta: metav1.ObjectMeta{Name: pvcName, Namespace: namespace}, Spec: corev1.PersistentVolumeClaimSpec{VolumeName: pvName}}
		pv := &corev1.PersistentVolume{ObjectMeta: metav1.ObjectMeta{Name: pvName}, Spec: corev1.PersistentVolumeSpec{AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce}}}
		pod := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: podName, Namespace: namespace}, Spec: corev1.PodSpec{Volumes: []corev1.Volume{{Name: "v", VolumeSource: corev1.VolumeSource{PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{ClaimName: pvcName}}}}}}
		clientset := fake.NewSimpleClientset(pvc, pv, pod)
		ctx := context.Background()
		canBeMounted, resultPodName, err := checkPVAccessMode(ctx, clientset, pvc, namespace)
		if err != nil {
			t.Errorf("checkPVAccessMode() error = %v, wantNil", err)
		}
		if canBeMounted || resultPodName != podName {
			t.Errorf("checkPVAccessMode() = (%v, %s), want (false, %s)", canBeMounted, resultPodName, podName)
		}
	})

	t.Run("RWX access mode", func(t *testing.T) {
		pvc := &corev1.PersistentVolumeClaim{ObjectMeta: metav1.ObjectMeta{Name: pvcName, Namespace: namespace}, Spec: corev1.PersistentVolumeClaimSpec{VolumeName: pvName}}
		pv := &corev1.PersistentVolume{ObjectMeta: metav1.ObjectMeta{Name: pvName}, Spec: corev1.PersistentVolumeSpec{AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteMany}}}
		clientset := fake.NewSimpleClientset(pvc, pv)
		ctx := context.Background()
		canBeMounted, resultPodName, err := checkPVAccessMode(ctx, clientset, pvc, namespace)
		if err != nil {
			t.Errorf("checkPVAccessMode() error = %v, wantNil", err)
		}
		if !canBeMounted || resultPodName != "" {
			t.Errorf("checkPVAccessMode() = (%v, %s), want (true, \"\")", canBeMounted, resultPodName)
		}
	})

	t.Run("Getting PV fails", func(t *testing.T) {
		pvc := &corev1.PersistentVolumeClaim{ObjectMeta: metav1.ObjectMeta{Name: pvcName, Namespace: namespace}, Spec: corev1.PersistentVolumeClaimSpec{VolumeName: "non-existent-pv"}}
		clientset := fake.NewSimpleClientset(pvc)
		clientset.PrependReactor("get", "persistentvolumes", func(action k8stesting.Action) (bool, runtime.Object, error) {
			return true, nil, fmt.Errorf("simulated API error getting PV")
		})
		ctx := context.Background()
		_, _, err := checkPVAccessMode(ctx, clientset, pvc, namespace)
		if err == nil {
			t.Errorf("checkPVAccessMode() error = nil, want non-nil for getting PV fails")
		}
	})

	t.Run("Listing pods fails", func(t *testing.T) {
		pvc := &corev1.PersistentVolumeClaim{ObjectMeta: metav1.ObjectMeta{Name: pvcName, Namespace: namespace}, Spec: corev1.PersistentVolumeClaimSpec{VolumeName: pvName}}
		pv := &corev1.PersistentVolume{ObjectMeta: metav1.ObjectMeta{Name: pvName}, Spec: corev1.PersistentVolumeSpec{AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce}}}
		clientset := fake.NewSimpleClientset(pvc, pv)
		clientset.PrependReactor("list", "pods", func(action k8stesting.Action) (bool, runtime.Object, error) {
			return true, nil, fmt.Errorf("simulated API error listing pods")
		})
		ctx := context.Background()
		_, _, err := checkPVAccessMode(ctx, clientset, pvc, namespace)
		if err == nil {
			t.Errorf("checkPVAccessMode() error = nil, want non-nil for listing pods fails")
		}
	})
}

func TestGetSecurityContext(t *testing.T) {
	t.Run("needsRoot is true", func(t *testing.T) {
		sc := getSecurityContext(true)
		if sc == nil {
			t.Fatal("getSecurityContext(true) returned nil")
		}
		if sc.AllowPrivilegeEscalation == nil || !*sc.AllowPrivilegeEscalation {
			t.Errorf("AllowPrivilegeEscalation got %v, want true", sc.AllowPrivilegeEscalation)
		}
		if sc.Capabilities == nil || len(sc.Capabilities.Add) != 2 || sc.Capabilities.Add[0] != "SYS_ADMIN" || sc.Capabilities.Add[1] != "SYS_CHROOT" {
			t.Errorf("Capabilities.Add got %v, want [SYS_ADMIN, SYS_CHROOT]", sc.Capabilities.Add)
		}
		if sc.RunAsUser != nil && *sc.RunAsUser != 0 {
			t.Errorf("RunAsUser got %v, want 0 or nil", sc.RunAsUser)
		}
		if sc.RunAsGroup != nil && *sc.RunAsGroup != 0 {
			t.Errorf("RunAsGroup got %v, want 0 or nil", sc.RunAsGroup)
		}
		if sc.RunAsNonRoot != nil && *sc.RunAsNonRoot {
			t.Errorf("RunAsNonRoot got %v, want false or nil", sc.RunAsNonRoot)
		}
	})

	t.Run("needsRoot is false", func(t *testing.T) {
		sc := getSecurityContext(false)
		if sc == nil {
			t.Fatal("getSecurityContext(false) returned nil")
		}
		if sc.AllowPrivilegeEscalation == nil || *sc.AllowPrivilegeEscalation {
			t.Errorf("AllowPrivilegeEscalation got %v, want false", sc.AllowPrivilegeEscalation)
		}
		if sc.RunAsUser == nil || *sc.RunAsUser != DefaultID {
			t.Errorf("RunAsUser got %v, want %d", sc.RunAsUser, DefaultID)
		}
		if sc.RunAsGroup == nil || *sc.RunAsGroup != DefaultID {
			t.Errorf("RunAsGroup got %v, want %d", sc.RunAsGroup, DefaultID)
		}
		if sc.RunAsNonRoot == nil || !*sc.RunAsNonRoot {
			t.Errorf("RunAsNonRoot got %v, want true", sc.RunAsNonRoot)
		}
		if sc.Capabilities == nil || len(sc.Capabilities.Drop) != 1 || sc.Capabilities.Drop[0] != "ALL" {
			t.Errorf("Capabilities.Drop got %v, want [ALL]", sc.Capabilities.Drop)
		}
	})
}

func TestGetEphemeralContainerSettings(t *testing.T) {
	t.Run("needsRoot is true", func(t *testing.T) {
		image, sc := getEphemeralContainerSettings(true)
		if image != PrivilegedImage {
			t.Errorf("Image got %s, want %s", image, PrivilegedImage)
		}
		if sc == nil {
			t.Fatal("SecurityContext is nil for needsRoot=true")
		}
		if sc.AllowPrivilegeEscalation == nil || !*sc.AllowPrivilegeEscalation {
			t.Error("AllowPrivilegeEscalation should be true for needsRoot=true")
		}
	})

	t.Run("needsRoot is false", func(t *testing.T) {
		image, sc := getEphemeralContainerSettings(false)
		if image != Image {
			t.Errorf("Image got %s, want %s", image, Image)
		}
		if sc == nil {
			t.Fatal("SecurityContext is nil for needsRoot=false")
		}
		if sc.AllowPrivilegeEscalation == nil || *sc.AllowPrivilegeEscalation {
			t.Error("AllowPrivilegeEscalation should be false for needsRoot=false")
		}
		if sc.RunAsUser == nil || *sc.RunAsUser != DefaultID {
			t.Errorf("RunAsUser got %v, want %d", sc.RunAsUser, DefaultID)
		}
	})
}

func TestCreateEphemeralContainer(t *testing.T) {
	namespace := "default"
	podName := "test-pod"
	pvcName := "test-pvc"
	volumeName := "test-volume"
	privateKeyValid := "dummyPrivateKey"
	publicKeyValid := "dummyPublicKey"
	proxyPodIPValid := "10.0.0.1"
	imageNameCustom := "custom-image"

	basePod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: podName, Namespace: namespace},
		Spec: corev1.PodSpec{
			Volumes: []corev1.Volume{
				{Name: volumeName, VolumeSource: corev1.VolumeSource{PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{ClaimName: pvcName}}},
			},
		},
	}
	
	var k8sClient kubernetes.Interface

	t.Run("Successful creation needsRoot false custom image", func(t *testing.T) {
		fakeClientset := fake.NewSimpleClientset(basePod.DeepCopy())
		fakeClientset.PrependReactor("patch", "pods", func(action k8stesting.Action) (bool, runtime.Object, error) {
			return true, basePod.DeepCopy(), nil
		})
		k8sClient = fakeClientset
		ctx := context.Background()
		err := createEphemeralContainer(ctx, k8sClient, namespace, podName, privateKeyValid, publicKeyValid, proxyPodIPValid, false, imageNameCustom)
		if err != nil {
			t.Fatalf("createEphemeralContainer() error = %v, wantNil", err)
		}

		actions := fakeClientset.Actions()
		if len(actions) < 2 {
			t.Fatalf("Expected at least 2 actions (Get, Patch), got %d: %v", len(actions), actions)
		}
		patchAction, ok := actions[1].(k8stesting.PatchAction)
		if !ok {
			t.Fatalf("Expected a patch action for actions[1], got %T", actions[1])
		}
		if patchAction.GetPatchType() != types.StrategicMergePatchType {
			t.Errorf("Patch action type = %s, want %s", patchAction.GetPatchType(), types.StrategicMergePatchType)
		}
		var patchContent map[string]interface{}
		if errJson := json.Unmarshal(patchAction.GetPatch(), &patchContent); errJson != nil {
			t.Fatalf("Failed to unmarshal patch: %v", errJson)
		}
		spec, _ := patchContent["spec"].(map[string]interface{})
		ephemeralContainers, _ := spec["ephemeralContainers"].([]interface{})
		ec := ephemeralContainers[0].(map[string]interface{})
		if ec["image"] != imageNameCustom {
			t.Errorf("EC image = %s, want %s", ec["image"], imageNameCustom)
		}
		env := ec["env"].([]interface{})
		foundNeedsRoot := false
		for _, e := range env {
			envVar := e.(map[string]interface{})
			if envVar["name"] == "NEEDS_ROOT" && envVar["value"] == "false" {
				foundNeedsRoot = true; break
			}
		}
		if !foundNeedsRoot {
			t.Errorf("NEEDS_ROOT env var not found or not false")
		}
	})

	t.Run("Successful creation needsRoot true default image", func(t *testing.T) {
		fakeClientset := fake.NewSimpleClientset(basePod.DeepCopy())
		fakeClientset.PrependReactor("patch", "pods", func(action k8stesting.Action) (bool, runtime.Object, error) {
			return true, basePod.DeepCopy(), nil
		})
		k8sClient = fakeClientset
		ctx := context.Background()
		err := createEphemeralContainer(ctx, k8sClient, namespace, podName, privateKeyValid, publicKeyValid, proxyPodIPValid, true, "")
		if err != nil {
			t.Fatalf("createEphemeralContainer() error = %v, wantNil", err)
		}
		actions := fakeClientset.Actions()
		patchAction := actions[1].(k8stesting.PatchAction)
		var patchContent map[string]interface{}
		json.Unmarshal(patchAction.GetPatch(), &patchContent)
		spec := patchContent["spec"].(map[string]interface{})
		ephemeralContainers := spec["ephemeralContainers"].([]interface{})
		ec := ephemeralContainers[0].(map[string]interface{})
		if ec["image"] != PrivilegedImage {
			t.Errorf("EC image = %s, want %s", ec["image"], PrivilegedImage)
		}
	})

	t.Run("Failure when getting existing pod fails", func(t *testing.T) {
		fakeClientset := fake.NewSimpleClientset() 
		fakeClientset.PrependReactor("get", "pods", func(action k8stesting.Action) (bool, runtime.Object, error) {
			return true, nil, fmt.Errorf("get pod error")
		})
		k8sClient = fakeClientset
		ctx := context.Background()
		err := createEphemeralContainer(ctx, k8sClient, namespace, podName, privateKeyValid, publicKeyValid, proxyPodIPValid, false, imageNameCustom)
		if err == nil {
			t.Error("createEphemeralContainer() error = nil, want error for get pod failure")
		}
	})

	t.Run("Failure when getPVCVolumeName fails", func(t *testing.T) {
		podNoPVC := basePod.DeepCopy()
		podNoPVC.Spec.Volumes = []corev1.Volume{} 
		fakeClientset := fake.NewSimpleClientset(podNoPVC)
		k8sClient = fakeClientset
		ctx := context.Background()
		err := createEphemeralContainer(ctx, k8sClient, namespace, podName, privateKeyValid, publicKeyValid, proxyPodIPValid, false, imageNameCustom)
		if err == nil {
			t.Error("createEphemeralContainer() error = nil, want error for getPVCVolumeName failure")
		}
		if !strings.Contains(err.Error(), "failed to find volume name") {
			t.Errorf("Error message mismatch: got '%s', want it to contain 'failed to find volume name'", err.Error())
		}
	})

	t.Run("Failure when marshalling ephemeral container spec fails", func(t *testing.T) {
		t.Skip("Skipping test for marshalling failure: Inducing json.Marshal failure is non-trivial with current function signature and types without deeper mocking.")
	})

	t.Run("Failure when patching pod fails", func(t *testing.T) {
		fakeClientset := fake.NewSimpleClientset(basePod.DeepCopy())
		fakeClientset.PrependReactor("patch", "pods", func(action k8stesting.Action) (bool, runtime.Object, error) {
			return true, nil, fmt.Errorf("patch pod error")
		})
		k8sClient = fakeClientset
		ctx := context.Background()
		err := createEphemeralContainer(ctx, k8sClient, namespace, podName, privateKeyValid, publicKeyValid, proxyPodIPValid, false, imageNameCustom)
		if err == nil {
			t.Error("createEphemeralContainer() error = nil, want error for patch pod failure")
		}
	})
}

func TestWaitForPodReady(t *testing.T) {
	namespace := "default"
	podName := "test-pod"
	var k8sClient kubernetes.Interface

	t.Run("Pod becomes ready", func(t *testing.T) {
		getCount := 0
		fakeClientset := fake.NewSimpleClientset()
		fakeClientset.PrependReactor("get", "pods", func(action k8stesting.Action) (handled bool, ret runtime.Object, err error) {
			getCount++
			pod := &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{Name: podName, Namespace: namespace},
				Status: corev1.PodStatus{
					Conditions: []corev1.PodCondition{
						{Type: corev1.PodReady, Status: corev1.ConditionFalse},
					},
				},
			}
			if getCount >= 2 { // Becomes ready on the 2nd get attempt
				pod.Status.Conditions[0].Status = corev1.ConditionTrue
			}
			return true, pod, nil
		})
		k8sClient = fakeClientset

		// waitForPodReady polls every 1 second. Timeout should allow for a few polls.
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()

		err := waitForPodReady(ctx, k8sClient, namespace, podName)
		if err != nil {
			t.Errorf("waitForPodReady() returned error: %v, want nil. GetCount: %d", err, getCount)
		}
	})

	t.Run("Pod does not become ready (timeout by context)", func(t *testing.T) {
		fakeClientset := fake.NewSimpleClientset()
		fakeClientset.PrependReactor("get", "pods", func(action k8stesting.Action) (handled bool, ret runtime.Object, err error) {
			pod := &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{Name: podName, Namespace: namespace},
				Status: corev1.PodStatus{
					Conditions: []corev1.PodCondition{
						{Type: corev1.PodReady, Status: corev1.ConditionFalse},
					},
				},
			}
			return true, pod, nil
		})
		k8sClient = fakeClientset
		
		// Timeout is 50ms, much shorter than waitForPodReady's internal 1s poll.
		ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
		defer cancel()

		err := waitForPodReady(ctx, k8sClient, namespace, podName)
		if err == nil {
			t.Errorf("waitForPodReady() returned nil error, want context.DeadlineExceeded")
		} else if !errors.Is(err, context.DeadlineExceeded) {
			t.Errorf("waitForPodReady() returned error type %T: %v, want context.DeadlineExceeded", err, err)
		}
	})

	t.Run("Getting pod fails initially", func(t *testing.T) {
		getCount := 0
		fakeClientset := fake.NewSimpleClientset()
		expectedError := fmt.Errorf("simulated API error getting pod")
		
		fakeClientset.PrependReactor("get", "pods", func(action k8stesting.Action) (handled bool, ret runtime.Object, err error) {
			getCount++
			if getCount == 1 { // Fail only on the first attempt
				return true, nil, expectedError
			}
			// Should not be reached if the first error is returned by PollUntilContextTimeout
			pod := &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{Name: podName, Namespace: namespace},
				Status: corev1.PodStatus{
					Conditions: []corev1.PodCondition{{Type: corev1.PodReady, Status: corev1.ConditionFalse}},
				},
			}
			return true, pod, nil 
		})
		k8sClient = fakeClientset

		// Context timeout needs to be > 1s for the first poll attempt to occur.
		ctx, cancel := context.WithTimeout(context.Background(), 1500*time.Millisecond) 
		defer cancel()

		err := waitForPodReady(ctx, k8sClient, namespace, podName)
		if err == nil {
			t.Errorf("waitForPodReady() returned nil error, want '%v'", expectedError)
		} else if !strings.Contains(err.Error(), expectedError.Error()) { // PollUntilContextTimeout wraps the error
			t.Errorf("waitForPodReady() returned error: %v, want error containing '%s'", err, expectedError.Error())
		}
	})
}


func TestGetPodIP(t *testing.T) {
	namespace := "default"
	podName := "test-pod"
	podIP := "192.168.1.1"

	t.Run("Pod exists", func(t *testing.T) {
		clientset := fake.NewSimpleClientset(&corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{Name: podName, Namespace: namespace},
			Status:     corev1.PodStatus{PodIP: podIP},
		})
		ctx := context.Background()
		ip, err := getPodIP(ctx, clientset, namespace, podName)
		if err != nil {
			t.Errorf("getPodIP() error = %v, wantNil", err)
		}
		if ip != podIP {
			t.Errorf("getPodIP() IP = %s, want %s", ip, podIP)
		}
	})

	t.Run("Pod does not exist", func(t *testing.T) {
		clientset := fake.NewSimpleClientset()
		ctx := context.Background()
		_, err := getPodIP(ctx, clientset, namespace, podName)
		if err == nil {
			t.Error("getPodIP() error = nil, want error for non-existent pod")
		}
	})

	t.Run("API error", func(t *testing.T) {
		clientset := fake.NewSimpleClientset()
		clientset.PrependReactor("get", "pods", func(action k8stesting.Action) (bool, runtime.Object, error) {
			return true, nil, fmt.Errorf("API error")
		})
		ctx := context.Background()
		_, err := getPodIP(ctx, clientset, namespace, podName)
		if err == nil {
			t.Error("getPodIP() error = nil, want error for API error")
		}
	})
}

func TestContains(t *testing.T) {
	modes := []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce, corev1.ReadWriteMany}
	if !contains(modes, corev1.ReadWriteOnce) {
		t.Error("Expected mode ReadWriteOnce to be found")
	}
	if contains(modes, corev1.ReadOnlyMany) {
		t.Error("Did not expect mode ReadOnlyMany to be found")
	}
}

func TestGeneratePodNameAndPort(t *testing.T) {
	name1, port1 := generatePodNameAndPort("standalone")
	name2, port2 := generatePodNameAndPort("standalone")
	if name1 == name2 {
		t.Error("Expected different pod names for subsequent calls")
	}
	if port1 == port2 {
		t.Logf("Warning: Generated ports are the same (%d), which is statistically unlikely but possible.", port1)
	}
}

func TestCreatePodSpec(t *testing.T) {
	podSpec := createPodSpec("test-pod", 12345, "test-pvc", "publicKey", "standalone", 22, "", false, "whatever", "secret", "300m")
	if podSpec.Name != "test-pod" {
		t.Errorf("Expected pod name 'test-pod', got '%s'", podSpec.Name)
	}
}

func TestGetPVCVolumeName(t *testing.T) {
	podWithPVC := &corev1.Pod{
		Spec: corev1.PodSpec{
			Volumes: []corev1.Volume{
				{Name: "test-volume", VolumeSource: corev1.VolumeSource{PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{ClaimName: "test-pvc"}}},
			},
		},
	}
	volumeName, err := getPVCVolumeName(podWithPVC)
	if err != nil {
		t.Errorf("getPVCVolumeName returned an error: %v for pod with PVC", err)
	}
	if volumeName != "test-volume" {
		t.Errorf("Expected volume name 'test-volume', got '%s'", volumeName)
	}

	podNoPVC := &corev1.Pod{
		Spec: corev1.PodSpec{
			Volumes: []corev1.Volume{
				{Name: "empty-dir-volume", VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{}}},
			},
		},
	}
	_, err = getPVCVolumeName(podNoPVC)
	if err == nil {
		t.Error("getPVCVolumeName should have returned an error for pod with no PVC volume")
	}
}
