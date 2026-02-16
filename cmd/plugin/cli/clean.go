// Package cli implements the command-line interface for pv-mounter.
package cli

import (
	"context"
	"fmt"

	"github.com/fenio/pv-mounter/pkg/plugin"
	"github.com/spf13/cobra"
)

func cleanCmd() *cobra.Command {
	var backend string

	cmd := &cobra.Command{
		Use:   "clean [flags] <namespace> <pvc-name> <local-mount-point>",
		Short: "Clean up resources created by pv-mounter",
		Long: `Clean up resources created by pv-mounter.

This command unmounts the local directory, terminates port-forwarding,
removes any standalone pods created, and cleans up ephemeral containers.`,
		Args: cobra.ExactArgs(3),
		RunE: func(_ *cobra.Command, args []string) error {
			backend = stringEnv("BACKEND", backend)

			if backend != "" && backend != "ssh" && backend != "nfs" {
				return fmt.Errorf("invalid value for --backend: %s (must be 'ssh' or 'nfs')", backend)
			}

			namespace := args[0]
			pvcName := args[1]
			localMountPoint := args[2]

			ctx := context.Background()
			if err := plugin.Clean(ctx, namespace, pvcName, localMountPoint, backend); err != nil {
				return fmt.Errorf("failed to clean PVC: %w", err)
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&backend, "backend", "", "Mount backend: 'ssh' (default) or 'nfs'")

	cmd.SetUsageTemplate(`
Usage:
  kubectl pv-mounter clean [flags] <namespace> <pvc-name> <local-mount-point>

Arguments:
  namespace             Kubernetes namespace containing the PVC
  pvc-name              Name of the PersistentVolumeClaim to clean up
  local-mount-point     Local directory path where the PVC was mounted

Flags:
{{.LocalFlags.FlagUsages | trimTrailingWhitespaces}}

Examples:
  # Clean resources associated with 'my-pvc' mounted at '/mnt/data'
  kubectl pv-mounter clean default my-pvc /mnt/data

  # Clean NFS-mounted resources
  kubectl pv-mounter clean --backend nfs default my-pvc /mnt/data

Environment Variables:
  BACKEND          Specify mount backend: 'ssh' (default) or 'nfs'

Notes:
  - This command attempts to remove all resources created by 'mount', including standalone pods and port forwarding.
  - Ephemeral containers cannot be fully deleted due to Kubernetes limitations. However, their processes will be terminated.
`)

	return cmd
}
