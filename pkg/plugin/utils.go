package plugin

import (
	crand "crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"

	"fmt"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"math/rand"
	"os"
	"os/exec"
	"runtime"
)

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

func randSeq(n int) string {
	letters := []rune("abcdefghijklmnopqrstuvwxyz0123456789")
	b := make([]rune, n)
	for i := range b {
		b[i] = letters[rand.Intn(len(letters))]
	}
	return string(b)
}

func GenerateKeyPair(bits int) (string, string, error) {
	// Generate a new private key
	privateKey, err := rsa.GenerateKey(crand.Reader, bits)
	if err != nil {
		return "", "", fmt.Errorf("failed to generate private key: %v", err)
	}

	// Encode the private key to PKCS8 format
	privateKeyPKCS8, err := x509.MarshalPKCS8PrivateKey(privateKey)
	if err != nil {
		return "", "", fmt.Errorf("failed to marshal private key to PKCS8: %v", err)
	}

	// Encode the private key to PEM format
	privateKeyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "PRIVATE KEY",
		Bytes: privateKeyPKCS8,
	})

	// Extract the public key from the private key
	publicKey := &privateKey.PublicKey

	// Encode the public key to PKIX, ASN.1 DER form
	pubASN1, err := x509.MarshalPKIXPublicKey(publicKey)
	if err != nil {
		return "", "", fmt.Errorf("failed to marshal public key: %v", err)
	}

	// Encode the public key to PEM format
	publicKeyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "PUBLIC KEY",
		Bytes: pubASN1,
	})

	return string(privateKeyPEM), string(publicKeyPEM), nil
}

func checkSSHFS() {
	_, err := exec.LookPath("sshfs")
	if err != nil {
		fmt.Println("sshfs is not available in your environment.")
		if runtime.GOOS == "darwin" {
			fmt.Println("For macOS, please install sshfs by visiting: https://osxfuse.github.io/")
		} else if runtime.GOOS == "linux" {
			fmt.Println("For Linux, please install sshfs by visiting: https://github.com/libfuse/sshfs")
		} else {
			fmt.Println("Please install sshfs and try again.")
		}
		os.Exit(1)
	}
}
