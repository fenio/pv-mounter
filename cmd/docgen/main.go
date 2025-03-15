package main

import (
	"github.com/fenio/pv-mounter/cmd/plugin/cli"
	"github.com/spf13/cobra/doc"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"os"
	"path/filepath"
)

func main() {
	// Create docs directory if not exists
	docsDir := "./docs/commands"
	if err := os.MkdirAll(docsDir, 0755); err != nil {
		panic(err)
	}

	// Initialize the root command with Kubernetes flags
	rootCmd := cli.RootCmd()
	cli.KubernetesConfigFlags = genericclioptions.NewConfigFlags(true)
	
	// Generate markdown docs
	err := doc.GenMarkdownTree(rootCmd, docsDir)
	if err != nil {
		panic(err)
	}
	
	// Rename root file to match command name
	os.Rename(
		filepath.Join(docsDir, "pv-mounter.md"),
		filepath.Join(docsDir, "README.md"),
	)
}
