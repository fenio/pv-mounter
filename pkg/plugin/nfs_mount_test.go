package plugin

import (
	"context"
	"runtime"
	"testing"
)

func TestIsNFSReady(t *testing.T) {
	t.Run("Port not listening returns false", func(t *testing.T) {
		ctx := context.Background()
		// Use a port that is very unlikely to be in use
		ready := isNFSReady(ctx, 19999)
		if ready {
			t.Error("Expected isNFSReady to return false for a port that is not listening")
		}
	})
}

func TestBuildNFSMountCommand(t *testing.T) {
	ctx := context.Background()

	t.Run("Build NFS mount command with correct arguments", func(t *testing.T) {
		cmd := buildNFSMountCommand(ctx, "/mnt/pvc", 12345)

		args := cmd.Args

		if runtime.GOOS == "darwin" {
			expectedArgs := []string{
				"mount",
				"-t", "nfs",
				"-o", "nfsvers=4,port=12345,tcp",
				"localhost:/volume",
				"/mnt/pvc",
			}

			if len(args) != len(expectedArgs) {
				t.Fatalf("Expected %d args, got %d: %v", len(expectedArgs), len(args), args)
			}

			for i, arg := range expectedArgs {
				if args[i] != arg {
					t.Errorf("Arg %d: expected '%s', got '%s'", i, arg, args[i])
				}
			}
		} else {
			expectedArgs := []string{
				"mount", "-t", "nfs4",
				"-o", "port=12345,vers=4.2",
				"localhost:/volume",
				"/mnt/pvc",
			}

			if len(args) != len(expectedArgs) {
				t.Fatalf("Expected %d args, got %d: %v", len(expectedArgs), len(args), args)
			}

			for i, arg := range expectedArgs {
				if args[i] != arg {
					t.Errorf("Arg %d: expected '%s', got '%s'", i, arg, args[i])
				}
			}
		}
	})
}
