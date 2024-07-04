package cli

import (
	"github.com/fenio/pv-mounter/pkg/plugin"
	"github.com/spf13/cobra"
	"log"
	"os"
	"strconv"
)

func mountCmd() *cobra.Command {
	var needsRoot bool
	var debug bool

	cmd := &cobra.Command{
		Use:   "mount [--needs-root] [--debug] <namespace> <pvc-name> <local-mount-point>",
		Short: "Mount a PVC to a local directory",
		Args:  cobra.ExactArgs(3),
		RunE: func(cmd *cobra.Command, args []string) error {
			// Check for NEEDS_ROOT environment variable
			if needsRootEnv, exists := os.LookupEnv("NEEDS_ROOT"); exists {
				// Convert the environment variable to a boolean
				if parsedNeedsRoot, err := strconv.ParseBool(needsRootEnv); err == nil {
					needsRoot = parsedNeedsRoot
				} else {
					log.Fatalf("Invalid value for NEEDS_ROOT: %v", needsRootEnv)
				}
			}

			// Check for DEBUG environment variable
			if debugEnv, exists := os.LookupEnv("DEBUG"); exists {
				// Convert the environment variable to a boolean
				if parsedDebug, err := strconv.ParseBool(debugEnv); err == nil {
					debug = parsedDebug
				} else {
					log.Fatalf("Invalid value for DEBUG: %v", debugEnv)
				}
			}

			namespace := args[0]
			pvcName := args[1]
			localMountPoint := args[2]
			if err := plugin.Mount(namespace, pvcName, localMountPoint, needsRoot, debug); err != nil {
				log.Fatalf("Failed to mount PVC: %v", err)
			}
			return nil
		},
	}

	cmd.Flags().BoolVar(&needsRoot, "needs-root", false, "Mount the filesystem using the root account")
	cmd.Flags().BoolVar(&debug, "debug", false, "Enable debug mode to print additional information")
	return cmd
}
