// Package main is the entry point for the pv-mounter kubectl plugin.
package main

import (
	"github.com/fenio/pv-mounter/cmd/plugin/cli"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp" // required for GKE
)

func main() {
	cli.InitAndExecute()
}
