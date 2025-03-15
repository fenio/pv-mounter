package main

import (
	"github.com/fenio/pv-mounter/cmd/plugin/cli"
	"github.com/spf13/cobra/doc"
)

func main() {
	rootCmd := cli.RootCmd()
	// Generate markdown docs
	err := doc.GenMarkdownTree(rootCmd, "./docs/commands")
	if err != nil {
		panic(err)
	}
}
