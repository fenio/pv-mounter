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
)

func RootCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "pv-mounter",
		Short: "A tool to mount and unmount PVs",
		Long:  `A tool to mount and unmount PVs using SSHFS.`,
	}

	if strings.HasPrefix(filepath.Base(os.Args[0]), "kubectl-") {
		cmd.Use = "kubectl pv-mounter [flags]"
	}

	cmd.AddCommand(mountCmd())
	cmd.AddCommand(cleanCmd())

	return cmd
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
