package cli

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"k8s.io/cli-runtime/pkg/genericclioptions"
)

var KubernetesConfigFlags *genericclioptions.ConfigFlags

func RootCmd() *cobra.Command {
	rootCmd := &cobra.Command{
		Use:   "pv-mounter",
		Short: "A tool to mount and unmount Kubernetes PersistentVolumes using SSHFS",
		Long: `pv-mounter is a kubectl plugin that allows you to easily mount and unmount
Kubernetes PersistentVolumeClaims (PVCs) locally via SSHFS.

It handles the creation of proxy pods, ephemeral containers, port-forwarding,
and SSHFS connections transparently.`,
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
`,
	}

	rootCmd.AddCommand(mountCmd())
	rootCmd.AddCommand(cleanCmd())

	return rootCmd
}

func InitAndExecute() {
	KubernetesConfigFlags = genericclioptions.NewConfigFlags(true)
	KubernetesConfigFlags.AddFlags(RootCmd().PersistentFlags())

	viper.AutomaticEnv()
	viper.SetEnvKeyReplacer(strings.NewReplacer("-", "_"))

	if err := RootCmd().Execute(); err != nil {
		os.Exit(1)
	}
}
