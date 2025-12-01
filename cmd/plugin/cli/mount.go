package cli

import (
	"context"
	"fmt"
	"os"
	"strconv"

	"github.com/fenio/pv-mounter/pkg/plugin"
	"github.com/spf13/cobra"
)

func parseBoolEnv(envName string, currentValue bool) (bool, error) {
	env, exists := os.LookupEnv(envName)
	if !exists {
		return currentValue, nil
	}
	parsed, err := strconv.ParseBool(env)
	if err != nil {
		return false, fmt.Errorf("invalid value for %s: %v", envName, env)
	}
	return parsed, nil
}

func getStringEnv(envName string, currentValue string) string {
	if currentValue != "" {
		return currentValue
	}
	if env, exists := os.LookupEnv(envName); exists {
		return env
	}
	return currentValue
}

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
		RunE: func(_ *cobra.Command, args []string) error {
			var err error

			needsRoot, err = parseBoolEnv("NEEDS_ROOT", needsRoot)
			if err != nil {
				return err
			}

			debug, err = parseBoolEnv("DEBUG", debug)
			if err != nil {
				return err
			}

			image = getStringEnv("IMAGE", image)
			imageSecret = getStringEnv("IMAGE_SECRET", imageSecret)
			cpuLimit = getStringEnv("CPU_LIMIT", cpuLimit)

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
