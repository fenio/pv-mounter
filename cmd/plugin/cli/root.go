package cli

import (
	"os"
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
		Short: "A plugin to mount and clean PVCs",
		Long:  `A plugin to mount and clean PVCs using SSHFS.`,
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
