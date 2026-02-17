package plugin

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"runtime"
	"time"
)

const (
	// DefaultNFSPort is the default NFS port
	DefaultNFSPort int = 2049

	// NFSImageVersion specifies the container image version for nfs-ganesha
	NFSImageVersion = "latest"

	// NFSImage is the default NFS container image
	NFSImage = "bfenski/nfs-ganesha:" + NFSImageVersion
)

// setupNFSPortForwarding establishes port forwarding to a pod's NFS port.
func setupNFSPortForwarding(ctx context.Context, namespace, podName string, port int, debug bool, timeout time.Duration) (*exec.Cmd, error) {
	cmd := exec.CommandContext(ctx, "kubectl", "port-forward", fmt.Sprintf("pod/%s", podName), fmt.Sprintf("%d:%d", port, DefaultNFSPort), "-n", namespace) // #nosec G204 -- namespace and podName are validated Kubernetes resource names
	if debug {
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
	}
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start port-forward: %w", err)
	}

	if err := waitForNFSReady(ctx, port, timeout); err != nil {
		cleanupPortForward(cmd)
		return nil, fmt.Errorf("failed to establish NFS connection: %w", err)
	}

	if !debug {
		fmt.Printf("Forwarding from 127.0.0.1:%d -> %d\n", port, DefaultNFSPort)
	}
	return cmd, nil
}

// waitForNFSReady waits for NFS daemon to become available on the specified port.
func waitForNFSReady(ctx context.Context, port int, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			if time.Now().After(deadline) {
				return fmt.Errorf("timeout waiting for NFS daemon to become ready on port %d", port)
			}

			if isNFSReady(ctx, port) {
				return nil
			}
		}
	}
}

// isNFSReady checks if the NFS daemon is reachable through the port-forward.
// A simple TCP connect is not sufficient because kubectl port-forward accepts
// local connections immediately before the remote tunnel is established.
// Instead, we connect and attempt a read with a short timeout: if the
// connection is properly forwarded to Ganesha, the read will timeout
// (Ganesha waiting for client data). If the forward failed, the connection
// will be closed immediately with an error.
func isNFSReady(ctx context.Context, port int) bool {
	dialer := &net.Dialer{Timeout: time.Second}
	conn, err := dialer.DialContext(ctx, "tcp", fmt.Sprintf("127.0.0.1:%d", port))
	if err != nil {
		return false
	}
	defer func() { _ = conn.Close() }()

	if err := conn.SetReadDeadline(time.Now().Add(500 * time.Millisecond)); err != nil {
		return false
	}
	buf := make([]byte, 1)
	_, err = conn.Read(buf)
	if err != nil {
		var netErr net.Error
		if errors.As(err, &netErr) && netErr.Timeout() {
			return true // read timed out = connection alive, Ganesha is waiting
		}
		return false // connection closed = forward failed
	}
	return true
}

// mountPVCOverNFS mounts a PVC using NFS with retries.
// Ganesha may need time to fully initialize after the TCP port is listening.
func mountPVCOverNFS(ctx context.Context, port int, localMountPoint, pvcName string) error {
	const maxRetries = 5
	var lastErr error

	for attempt := range maxRetries {
		if attempt > 0 {
			time.Sleep(3 * time.Second)
		}

		mountCmd := buildNFSMountCommand(ctx, localMountPoint, port)
		mountCmd.Stdout = os.Stdout
		mountCmd.Stderr = os.Stderr

		if err := mountCmd.Run(); err != nil {
			lastErr = err
			continue
		}

		fmt.Printf("PVC %s mounted successfully to %s\n", pvcName, localMountPoint)
		return nil
	}

	return fmt.Errorf("failed to mount PVC using NFS: %w", lastErr)
}

// buildNFSMountCommand constructs the platform-specific NFS mount command.
func buildNFSMountCommand(ctx context.Context, localMountPoint string, port int) *exec.Cmd {
	if runtime.GOOS == "darwin" {
		return exec.CommandContext(ctx, // #nosec G204 -- localMountPoint is user-provided, port is generated
			"mount",
			"-t", "nfs",
			"-o", fmt.Sprintf("nfsvers=4,port=%d,tcp", port),
			"127.0.0.1:/volume",
			localMountPoint,
		)
	}
	return exec.CommandContext(ctx, // #nosec G204 -- localMountPoint is user-provided, port is generated
		"mount", "-t", "nfs4",
		"-o", fmt.Sprintf("port=%d,vers=4.2,soft,timeo=50,retrans=2,retry=0", port),
		"127.0.0.1:/volume",
		localMountPoint,
	)
}

// setupNFSPortForwardAndMount establishes port forwarding and mounts the volume via NFS.
func setupNFSPortForwardAndMount(ctx context.Context, namespace, podName string, port int, localMountPoint, pvcName string, debug bool) error {
	timeout := 30 * time.Second
	pfCmd, err := setupNFSPortForwarding(ctx, namespace, podName, port, debug, timeout)
	if err != nil {
		return err
	}
	if err := mountPVCOverNFS(ctx, port, localMountPoint, pvcName); err != nil {
		cleanupPortForward(pfCmd)
		return err
	}
	return nil
}
