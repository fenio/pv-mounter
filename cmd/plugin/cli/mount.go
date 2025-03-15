package cli

import (
	"context"
	"fmt"
	"github.com/fenio/pv-mounter/pkg/plugin"
	"github.com/spf13/cobra"
	"os"
	"strconv"
)

func mountCmd() *cobra.Command {
	var needsRoot bool
	var debug bool
	var image string
	var imageSecret string
	cmd := &cobra.Command{
		Use:   "mount [--needs-root] [--debug] [--image] [--image-secret] <namespace> <pvc-name> <local-mount-point>",
		Short: "Mount a PVC to a local directory",
		Args:  cobra.ExactArgs(3),
		RunE: func(cmd *cobra.Command, args []string) error {
			if needsRootEnv, exists := os.LookupEnv("NEEDS_ROOT"); exists {
				if parsedNeedsRoot, err := strconv.ParseBool(needsRootEnv); err == nil {
					needsRoot = parsedNeedsRoot
				} else {
					return fmt.Errorf("invalid value for NEEDS_ROOT: %v", needsRootEnv)
				}
			}
			if debugEnv, exists := os.LookupEnv("DEBUG"); exists {
				if parsedDebug, err := strconv.ParseBool(debugEnv); err == nil {
					debug = parsedDebug
				} else {
					return fmt.Errorf("invalid value for DEBUG: %v", debugEnv)
				}
			}
			if imageEnv, exists := os.LookupEnv("IMAGE"); exists {
				image = imageEnv
			}
			if imageSecretEnv, exists := os.LookupEnv("IMAGE_SECRET"); exists {
				imageSecret = imageSecretEnv
			}
			namespace := args[0]
			pvcName := args[1]
			localMountPoint := args[2]
			ctx := context.Background()
			if err := plugin.Mount(ctx, namespace, pvcName, localMountPoint, needsRoot, debug, image, imageSecret); err != nil {
				return fmt.Errorf("failed to mount PVC: %w", err)
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&needsRoot, "needs-root", false, "Mount the filesystem using the root account")
	cmd.Flags().BoolVar(&debug, "debug", false, "Enable debug mode to print additional information")
	cmd.Flags().StringVar(&image, "image", "", "Custom container image for the volume-exposer")
	cmd.Flags().StringVar(&imageSecret, "image-secret", "", "Kubernetes secret name for accessing private registry")
	return cmd
}
