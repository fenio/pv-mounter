package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/fenio/pv-mounter/cmd/plugin/cli"
	"github.com/spf13/cobra"
	"github.com/spf13/cobra/doc"
	"k8s.io/cli-runtime/pkg/genericclioptions"
)

func main() {
	// Setup paths
	docsDir := filepath.Join("docs", "commands")
	
	// Clean and create directory
	if err := os.RemoveAll(docsDir); err != nil {
		panic(err)
	}
	if err := os.MkdirAll(docsDir, 0755); err != nil {
		panic(err)
	}

	// Initialize commands
	rootCmd := cli.RootCmd()
	cli.KubernetesConfigFlags = genericclioptions.NewConfigFlags(true)

	// Generate standard docs
	if err := doc.GenMarkdownTree(rootCmd, docsDir); err != nil {
		panic(err)
	}

	// Process generated files
	if err := filepath.Walk(docsDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return err
		}

		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}

		// Fix internal links
		updated := strings.ReplaceAll(string(content), ".md", "")
		return os.WriteFile(path, []byte(updated), 0644)
	}); err != nil {
		panic(err)
	}

	// Generate unified documentation
	if err := generateMainDoc(rootCmd, docsDir); err != nil {
		panic(err)
	}
}

func generateMainDoc(rootCmd *cobra.Command, docsDir string) error {
	mainDoc := filepath.Join("docs", "README.md")
	content := fmt.Sprintf("# PV Mounter Documentation\n\n%s\n", rootCmd.Long)

	// Add command tree
	content += "## Command Reference\n\n"
	content += "```\n"
	content += rootCmd.UsageString()
	content += "```\n\n"

	// Add generated file links using manual traversal
	content += "## Detailed Commands\n\n"
	var traverseCommands func(*cobra.Command, int)
	traverseCommands = func(cmd *cobra.Command, level int) {
		if cmd.HasParent() {
			link := filepath.Join("commands", strings.ReplaceAll(cmd.CommandPath(), " ", "_")+".md")
			content += fmt.Sprintf("- [%s](%s)\n", cmd.CommandPath(), link)
		}
		for _, child := range cmd.Commands() {
			traverseCommands(child, level+1)
		}
	}
	traverseCommands(rootCmd, 0)

	return os.WriteFile(mainDoc, []byte(content), 0644)
}