package main

import (
	"github.com/fenio/pv-mounter/cmd/plugin/cli"
	"github.com/spf13/cobra/doc"
	"log"
	"os"
)

func main() {
	// Define output directory for documentation
	outputDir := "./docs"
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		log.Fatalf("Failed to create docs directory: %v", err)
	}

	// Generate Markdown documentation for CLI commands
	rootCmd := cli.RootCmd()
	if err := doc.GenMarkdownTree(rootCmd, outputDir); err != nil {
		log.Fatalf("Failed to generate CLI documentation: %v", err)
	}

	log.Printf("CLI documentation generated successfully in %s", outputDir)
}

