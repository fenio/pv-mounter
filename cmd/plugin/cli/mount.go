package cli

import (
	"context"
	"fmt"
	"os"
	"strconv"

	"github.com/fenio/pv-mounter/pkg/plugin"
	"github.com/spf13/cobra"
)

func mountCmd() *cobra.Command {
	var needsRoot bool
	var debug bool
	var image string
	var imageSecret string
	var cpuLimit string

	cmd := &cobra.Command{
		Use:   "mount [flags] <namespace> <pvc-name> <local-mount-point>",
		Short: "Mount a PersistentVolumeClaim (PVC) to a local directory using SSHFS.",
		Long: `Mount a PersistentVolumeClaim (PVC) to a local directory using SSHFS.

This command sets up necessary Kubernetes resources and establishes an SSHFS connection
to mount the specified PVC locally.`,
		Args: cobra.ExactArgs(3),
		RunE: func(cmd *cobra.Command, args []string) error {
			if env, exists := os.LookupEnv("NEEDS_ROOT"); exists {
				if parsed, err := strconv.ParseBool(env); err == nil {
					needsRoot = parsed
				} else {
					return fmt.Errorf("invalid value for NEEDS_ROOT: %v", env)
				}
			}

			if env, exists := os.LookupEnv("DEBUG"); exists {
				if parsed, err := strconv.ParseBool(env); err == nil {
					debug = parsed
				} else {
					return fmt.Errorf("invalid value for DEBUG: %v", env)
				}
			}

			if env, exists := os.LookupEnv("IMAGE"); exists && image == "" {
				image = env
			}

			if env, exists := os.LookupEnv("IMAGE_SECRET"); exists && imageSecret == "" {
				imageSecret = env
			}

			if env, exists := os.LookupEnv("CPU_LIMIT"); exists && cpuLimit == "" {
				cpuLimit = env
			}

			namespace, pvcName, localMountPoint := args[0], args[1], args[2]
			ctx := context.Background()

			return plugin.Mount(ctx, namespace, pvcName, localMountPoint, needsRoot, debug, image, imageSecret, cpuLimit)
		},
	}

	cmd.Flags().BoolVar(&needsRoot, "needs-root", false, "Mount the filesystem using the root account (default: false)")
	cmd.Flags().BoolVar(&debug, "debug", false, "Enable debug mode to print additional information (default: false)")
	cmd.Flags().StringVar(&image, "image", "", "Custom container image for the volume-exposer (optional)")
	cmd.Flags().StringVar(&imageSecret, "image-secret", "", "Kubernetes secret name for accessing private registry (optional)")
	cmd.Flags().StringVar(&cpuLimit, "cpu-limit", "", "Set CPU limit for the volume-exposer container (optional)")

	cmd.SetUsageTemplate(`
Usage:
  kubectl pv-mounter mount [flags] <namespace> <pvc-name> <local-mount-point>

Arguments:
  namespace             Kubernetes namespace containing the PVC
  pvc-name              Name of the PersistentVolumeClaim to mount
  local-mount-point     Local directory path where the PVC will be mounted

Flags:
{{.LocalFlags.FlagUsages | trimTrailingWhitespaces}}

Examples:
  # Mount a PVC named 'my-pvc' from namespace 'default' to '/mnt/data'
  kubectl pv-mounter mount default my-pvc /mnt/data

  # Mount with root privileges and debug enabled
  kubectl pv-mounter mount --needs-root --debug default my-pvc /mnt/data

Environment Variables:
  NEEDS_ROOT       Set to 'true' to mount with root privileges by default
  DEBUG            Set to 'true' to enable debug mode by default
  IMAGE            Specify default custom container image
  IMAGE_SECRET     Specify default Kubernetes secret for private registry access
  CPU_LIMIT        Specify default CPU limit for the container (e.g., 200m)
`)

	return cmd
}
