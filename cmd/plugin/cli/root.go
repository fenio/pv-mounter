package cli

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"k8s.io/cli-runtime/pkg/genericclioptions"
)

var (
	KubernetesConfigFlags *genericclioptions.ConfigFlags
	rootCmd               *cobra.Command
)

func init() {
	rootCmd = &cobra.Command{
		Use:   "pv-mounter",
		Short: "A tool to mount and unmount PVs",
		Long:  `A tool to mount and unmount PVs using SSHFS.`,
	}

	if strings.HasPrefix(filepath.Base(os.Args[0]), "kubectl-") {
		rootCmd.Annotations = map[string]string{
			cobra.CommandDisplayNameAnnotation: "kubectl pv-mounter",
		}
	}

	rootCmd.AddCommand(mountCmd())
	rootCmd.AddCommand(cleanCmd())
}

func RootCmd() *cobra.Command {
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
