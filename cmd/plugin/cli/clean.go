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
		Short: "Clean the mounted PVC",
		Args:  cobra.ExactArgs(3),
		RunE: func(cmd *cobra.Command, args []string) error {
			namespace := args[0]
			pvcName := args[1]
			localMountPoint := args[2]

			// Create a context
			ctx := context.Background()

			if err := plugin.Clean(ctx, namespace, pvcName, localMountPoint); err != nil {
				return fmt.Errorf("failed to clean PVC: %w", err)
			}
			return nil
		},
	}
	return cmd
}
