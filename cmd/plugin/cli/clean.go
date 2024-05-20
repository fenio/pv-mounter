package cli

import (
	"github.com/fenio/pv-mounter/pkg/plugin"
	"github.com/spf13/cobra"
	"log"
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
			if err := plugin.Clean(namespace, pvcName, localMountPoint); err != nil {
				log.Fatalf("Failed to clean PVC: %v", err)
			}
			return nil
		},
	}
	return cmd
}
