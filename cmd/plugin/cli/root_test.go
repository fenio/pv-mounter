package cli

import (
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
}
