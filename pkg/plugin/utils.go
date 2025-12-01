package plugin

import (
	crand "crypto/rand"
	"crypto/x509"
	"encoding/pem"

	"crypto/ecdsa"
	"crypto/elliptic"

	"fmt"
	"math/big"
	"math/rand/v2"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"

	"golang.org/x/crypto/ssh"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

// kubernetesNameRegex is a precompiled regex for validating Kubernetes names.
// DNS-1123 subdomain naming rules: lowercase alphanumeric, '-', or '.', must start and end with alphanumeric.
var kubernetesNameRegex = regexp.MustCompile(`^[a-z0-9]([-a-z0-9.]*[a-z0-9])?$`)

// BuildKubeClient creates a Kubernetes clientset from the kubeconfig file.
func BuildKubeClient() (*kubernetes.Clientset, error) {
	kubeconfig := os.Getenv("KUBECONFIG")
	if kubeconfig == "" {
		home := os.Getenv("HOME")
		kubeconfig = filepath.Join(home, ".kube", "config")
	}

	config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		return nil, fmt.Errorf("failed to build Kubernetes config: %w", err)
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create Kubernetes client: %w", err)
	}

	return clientset, nil
}

func randSeq(n int) string {
	if n <= 0 {
		return ""
	}
	letters := []rune("abcdefghijklmnopqrstuvwxyz0123456789")
	b := make([]rune, n)
	for i := range b {
		idx, err := crand.Int(crand.Reader, big.NewInt(int64(len(letters))))
		if err != nil {
			b[i] = letters[rand.IntN(len(letters))] // #nosec G404 -- fallback only, crypto/rand is attempted first
		} else {
			b[i] = letters[idx.Int64()]
		}
	}
	return string(b)
}

// GenerateKeyPair generates an ECDSA key pair for SSH authentication.
func GenerateKeyPair(curve elliptic.Curve) (string, string, error) {
	if curve == nil {
		return "", "", fmt.Errorf("curve must not be nil")
	}
	// Generate a new private key
	privateKey, err := ecdsa.GenerateKey(curve, crand.Reader)
	if err != nil {
		return "", "", fmt.Errorf("failed to generate private key: %w", err)
	}

	// Encode the private key to PKCS8 format
	privateKeyPKCS8, err := x509.MarshalECPrivateKey(privateKey)
	if err != nil {
		return "", "", fmt.Errorf("failed to marshal private key to PKCS8: %w", err)
	}

	// Encode the private key to PEM format
	privateKeyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "EC PRIVATE KEY",
		Bytes: privateKeyPKCS8,
	})

	// Extract the public key from the private key
	publicKey := &privateKey.PublicKey

	// Convert the ECDSA public key to the ssh.PublicKey type
	sshPublicKey, err := ssh.NewPublicKey(publicKey)
	if err != nil {
		return "", "", fmt.Errorf("failed to create SSH public key: %w", err)
	}

	// Encode the SSH public key to the authorized_keys format
	publicKeyBytes := ssh.MarshalAuthorizedKey(sshPublicKey)
	trimmedPublicKey := strings.TrimSpace(string(publicKeyBytes))

	return string(privateKeyPEM), trimmedPublicKey, nil
}

func checkSSHFS() {
	_, err := exec.LookPath("sshfs")
	if err != nil {
		fmt.Println("sshfs is not available in your environment.")
		switch runtime.GOOS {
		case "darwin":
			fmt.Println("For macOS, please install sshfs by visiting: https://osxfuse.github.io/")
		case "linux":
			fmt.Println("For Linux, please install sshfs by visiting: https://github.com/libfuse/sshfs")
		default:
			fmt.Println("Please install sshfs and try again.")
		}
		os.Exit(1)
	}
}

// ValidateKubernetesName validates that a name conforms to Kubernetes naming rules.
// DNS-1123 subdomain naming rules allow dots in addition to alphanumeric characters and hyphens.
func ValidateKubernetesName(name, fieldName string) error {
	if name == "" {
		return fmt.Errorf("%s cannot be empty", fieldName)
	}
	if len(name) > 253 {
		return fmt.Errorf("%s must be no more than 253 characters", fieldName)
	}
	if !kubernetesNameRegex.MatchString(name) {
		return fmt.Errorf("%s must consist of lower case alphanumeric characters, '-', or '.', and must start and end with an alphanumeric character", fieldName)
	}
	return nil
}
