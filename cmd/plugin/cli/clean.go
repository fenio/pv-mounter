package cli

import (
	"context"
	"fmt"

	"github.com/fenio/pv-mounter/pkg/plugin"
	"github.com/spf13/cobra"
)

func cleanCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "clean <namespace> <pvc-name> <local-mount-point>",
		Short: "Clean up resources created by pv-mounter",
		Long: `Clean up resources created by pv-mounter.

This command unmounts the local directory, terminates port-forwarding,
removes any proxy pods created, and cleans up ephemeral containers.`,
		Args: cobra.ExactArgs(3),
		RunE: func(cmd *cobra.Command, args []string) error {
			namespace := args[0]
			pvcName := args[1]
			localMountPoint := args[2]

			ctx := context.Background()
			if err := plugin.Clean(ctx, namespace, pvcName, localMountPoint); err != nil {
				return fmt.Errorf("failed to clean PVC: %w", err)
			}

			return nil
		},
	}

	cmd.SetUsageTemplate(`
Usage:
  kubectl pv-mounter clean <namespace> <pvc-name> <local-mount-point>

Arguments:
  namespace             Kubernetes namespace containing the PVC
  pvc-name              Name of the PersistentVolumeClaim to clean up
  local-mount-point     Local directory path where the PVC was mounted

Examples:
  # Clean resources associated with 'my-pvc' mounted at '/mnt/data'
  kubectl pv-mounter clean default my-pvc /mnt/data

Notes:
  - This command attempts to remove all resources created by 'mount', including proxy pods and port forwarding.
  - Ephemeral containers cannot be fully deleted due to Kubernetes limitations. However, their processes will be terminated.
`)

	return cmd
}
