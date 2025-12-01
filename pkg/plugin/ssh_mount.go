package plugin

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/exec"
	"sync"
	"time"
)

var (
	tempKeyFiles   = make(map[string]struct{})
	tempKeyFilesMu sync.Mutex
)

// registerTempKeyFile registers a temporary key file for cleanup.
func registerTempKeyFile(path string) {
	tempKeyFilesMu.Lock()
	defer tempKeyFilesMu.Unlock()
	tempKeyFiles[path] = struct{}{}
}

// unregisterTempKeyFile removes a temporary key file from the cleanup list.
func unregisterTempKeyFile(path string) {
	tempKeyFilesMu.Lock()
	defer tempKeyFilesMu.Unlock()
	delete(tempKeyFiles, path)
}

// cleanupTempKeyFiles removes all registered temporary key files.
func cleanupTempKeyFiles() {
	tempKeyFilesMu.Lock()
	defer tempKeyFilesMu.Unlock()
	for file := range tempKeyFiles {
		_ = os.Remove(file)
	}
}

// setupPortForwarding establishes port forwarding to a pod.
func setupPortForwarding(ctx context.Context, namespace, podName string, port int, debug bool, timeout time.Duration) (*exec.Cmd, error) {
	cmd := exec.CommandContext(ctx, "kubectl", "port-forward", fmt.Sprintf("pod/%s", podName), fmt.Sprintf("%d:%d", port, DefaultSSHPort), "-n", namespace) // #nosec G204 -- namespace and podName are validated Kubernetes resource names
	if debug {
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
	}
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start port-forward: %w", err)
	}

	if err := waitForSSHReady(ctx, port, timeout); err != nil {
		cleanupPortForward(cmd)
		return nil, fmt.Errorf("failed to establish SSH connection: %w", err)
	}

	if !debug {
		fmt.Printf("Forwarding from 127.0.0.1:%d -> %d\n", port, DefaultSSHPort)
	}
	return cmd, nil
}

// cleanupPortForward terminates a port-forward process.
func cleanupPortForward(cmd *exec.Cmd) {
	if cmd != nil && cmd.Process != nil {
		_ = cmd.Process.Kill()
	}
}

// waitForSSHReady waits for SSH daemon to become available on the specified port.
func waitForSSHReady(ctx context.Context, port int, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			if time.Now().After(deadline) {
				return fmt.Errorf("timeout waiting for SSH daemon to become ready on port %d", port)
			}

			if isSSHReady(ctx, port) {
				return nil
			}
		}
	}
}

// isSSHReady checks if SSH daemon is ready by attempting to connect and read the SSH banner.
func isSSHReady(ctx context.Context, port int) bool {
	dialer := &net.Dialer{Timeout: time.Second}
	conn, err := dialer.DialContext(ctx, "tcp", fmt.Sprintf("127.0.0.1:%d", port))
	if err != nil {
		return false
	}
	defer conn.Close() //nolint:errcheck // Best effort cleanup, error not actionable

	buf := make([]byte, 4)
	if err := conn.SetReadDeadline(time.Now().Add(2 * time.Second)); err != nil {
		return false
	}

	n, err := conn.Read(buf)
	return err == nil && n >= 3 && string(buf[:3]) == "SSH"
}

// mountPVCOverSSH mounts a PVC using SSHFS.
func mountPVCOverSSH(
	ctx context.Context,
	port int,
	localMountPoint, pvcName, privateKey string,
	needsRoot bool) error {

	keyFilePath, cleanup, err := createTempSSHKeyFile(privateKey)
	if err != nil {
		return err
	}
	defer cleanup()

	sshUser := selectSSHUser(needsRoot)
	sshfsCmd := buildSSHFSCommand(ctx, keyFilePath, sshUser, localMountPoint, port)

	sshfsCmd.Stdout = os.Stdout
	sshfsCmd.Stderr = os.Stderr

	if err := sshfsCmd.Run(); err != nil {
		return fmt.Errorf("failed to mount PVC using SSHFS: %w", err)
	}

	fmt.Printf("PVC %s mounted successfully to %s\n", pvcName, localMountPoint)
	return nil
}

// createTempSSHKeyFile creates a temporary file containing the SSH private key.
func createTempSSHKeyFile(privateKey string) (string, func(), error) {
	tmpFile, err := os.CreateTemp("", "ssh_key_*.pem")
	if err != nil {
		return "", nil, fmt.Errorf("failed to create temporary file for SSH private key: %w", err)
	}
	keyFilePath := tmpFile.Name()
	registerTempKeyFile(keyFilePath)

	cleanup := func() {
		_ = os.Remove(keyFilePath)
		unregisterTempKeyFile(keyFilePath)
	}

	if err := os.Chmod(keyFilePath, 0600); err != nil {
		cleanup()
		return "", nil, fmt.Errorf("failed to set permissions on temporary SSH key file: %w", err)
	}

	if _, err := tmpFile.Write([]byte(privateKey)); err != nil {
		cleanup()
		return "", nil, fmt.Errorf("failed to write SSH private key to temporary file: %w", err)
	}
	if err := tmpFile.Close(); err != nil {
		cleanup()
		return "", nil, fmt.Errorf("failed to close temporary file: %w", err)
	}

	return keyFilePath, cleanup, nil
}

// selectSSHUser selects the SSH user based on whether root access is needed.
func selectSSHUser(needsRoot bool) string {
	if needsRoot {
		return "root"
	}
	return "ve"
}

// buildSSHFSCommand constructs the sshfs command with appropriate options.
func buildSSHFSCommand(ctx context.Context, keyFilePath, sshUser, localMountPoint string, port int) *exec.Cmd {
	return exec.CommandContext(ctx, // #nosec G204 -- keyFilePath is a securely created temp file, localMountPoint is user-provided
		"sshfs",
		"-o", fmt.Sprintf("IdentityFile=%s", keyFilePath),
		"-o", "StrictHostKeyChecking=no",
		"-o", "UserKnownHostsFile=/dev/null",
		"-o", "nomap=ignore",
		fmt.Sprintf("%s@localhost:/volume", sshUser),
		localMountPoint,
		"-p", fmt.Sprintf("%d", port),
	)
}
