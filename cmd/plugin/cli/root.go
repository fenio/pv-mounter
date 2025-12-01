package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"k8s.io/cli-runtime/pkg/genericclioptions"
)

// KubernetesConfigFlags holds the Kubernetes client configuration flags
var KubernetesConfigFlags *genericclioptions.ConfigFlags
var rootCmd *cobra.Command

// RootCmd returns the root cobra command for the pv-mounter CLI
func RootCmd() *cobra.Command {
	if rootCmd != nil {
		return rootCmd
	}

	rootCmd = &cobra.Command{
		Use:   "pv-mounter",
		Short: "Mount and unmount Kubernetes PersistentVolumes using SSHFS",
		Long: `pv-mounter is a kubectl plugin that allows you to easily mount and unmount
Kubernetes PersistentVolumeClaims (PVCs) locally via SSHFS.

It transparently manages proxy pods, ephemeral containers, port-forwarding,
and SSHFS connections.`,
	}

	if strings.HasPrefix(filepath.Base(os.Args[0]), "kubectl-") {
		rootCmd.Annotations = map[string]string{
			cobra.CommandDisplayNameAnnotation: "kubectl pv-mounter",
		}
	}

	rootCmd.AddCommand(mountCmd())
	rootCmd.AddCommand(cleanCmd())

	rootCmd.SetUsageTemplate(`
Usage:
  kubectl pv-mounter [command]

Available Commands:
{{range .Commands}}{{printf "  %-15s %s\n" .Name .Short}}{{end}}

Flags:
{{.PersistentFlags.FlagUsages | trimTrailingWhitespaces}}

Examples:
  # Mount a PVC named 'my-pvc' from namespace 'default' to '/mnt/data'
  kubectl pv-mounter mount default my-pvc /mnt/data

  # Clean resources associated with 'my-pvc'
  kubectl pv-mounter clean default my-pvc /mnt/data

Use "kubectl pv-mounter [command] --help" for more information about a command.
`)

	return rootCmd
}

// InitAndExecute initializes the CLI with Kubernetes config flags and executes the root command
func InitAndExecute() {
	KubernetesConfigFlags = genericclioptions.NewConfigFlags(true)
	KubernetesConfigFlags.AddFlags(RootCmd().PersistentFlags())

	viper.AutomaticEnv()
	viper.SetEnvKeyReplacer(strings.NewReplacer("-", "_"))

	if err := RootCmd().Execute(); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
