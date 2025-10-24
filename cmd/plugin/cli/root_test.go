package cli

import (
	"os"
	"testing"
)

func TestRootCmd(t *testing.T) {
	cmd := RootCmd()

	if cmd.Use != "pv-mounter" {
		t.Errorf("Expected Use to be 'pv-mounter', got '%s'", cmd.Use)
	}

	if cmd.Short == "" {
		t.Error("Expected Short description to be set")
	}

	if cmd.Long == "" {
		t.Error("Expected Long description to be set")
	}

	t.Run("Has mount subcommand", func(t *testing.T) {
		mountCmd := cmd.Commands()
		found := false
		for _, c := range mountCmd {
			if c.Name() == "mount" {
				found = true
				break
			}
		}
		if !found {
			t.Error("Expected mount subcommand to be registered")
		}
	})

	t.Run("Has clean subcommand", func(t *testing.T) {
		cleanCmd := cmd.Commands()
		found := false
		for _, c := range cleanCmd {
			if c.Name() == "clean" {
				found = true
				break
			}
		}
		if !found {
			t.Error("Expected clean subcommand to be registered")
		}
	})

	t.Run("RootCmd singleton", func(t *testing.T) {
		cmd1 := RootCmd()
		cmd2 := RootCmd()
		if cmd1 != cmd2 {
			t.Error("Expected RootCmd to return the same instance")
		}
	})
}

func TestRootCmdAnnotations(t *testing.T) {
	t.Run("Kubectl plugin annotation", func(t *testing.T) {
		originalArgs := os.Args
		os.Args = []string{"kubectl-pv-mounter"}
		rootCmd = nil
		defer func() {
			os.Args = originalArgs
			rootCmd = nil
		}()

		cmd := RootCmd()
		if cmd.Annotations != nil {
			if displayName, exists := cmd.Annotations["commandDisplayNameAnnotation"]; exists {
				if displayName != "kubectl pv-mounter" {
					t.Errorf("Expected display name 'kubectl pv-mounter', got '%s'", displayName)
				}
			}
		}
	})

	t.Run("No annotation for non-kubectl invocation", func(t *testing.T) {
		originalArgs := os.Args
		os.Args = []string{"pv-mounter"}
		rootCmd = nil
		defer func() {
			os.Args = originalArgs
			rootCmd = nil
		}()

		cmd := RootCmd()
		if cmd.Annotations != nil {
			if _, exists := cmd.Annotations["commandDisplayNameAnnotation"]; exists {
				t.Error("Expected no annotation for non-kubectl invocation")
			}
		}
	})
}
