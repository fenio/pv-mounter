package cli

import (
	"testing"
)

func TestCleanCmd(t *testing.T) {
	cmd := cleanCmd()

	if cmd.Use != "clean [flags] <namespace> <pvc-name> <local-mount-point>" {
		t.Errorf("Expected Use to be 'clean [flags] <namespace> <pvc-name> <local-mount-point>', got '%s'", cmd.Use)
	}

	if cmd.Short == "" {
		t.Error("Expected Short description to be set")
	}

	backendFlag := cmd.Flags().Lookup("backend")
	if backendFlag == nil {
		t.Error("Expected --backend flag to be defined")
	}
	if backendFlag != nil && backendFlag.DefValue != "" {
		t.Errorf("Expected --backend default to be empty, got '%s'", backendFlag.DefValue)
	}
}

func TestCleanCmdArgValidation(t *testing.T) {
	cmd := cleanCmd()

	t.Run("Too few arguments", func(t *testing.T) {
		cmd.SetArgs([]string{"default", "test-pvc"})
		err := cmd.Execute()
		if err == nil {
			t.Error("Expected error when too few arguments are provided")
		}
	})

	t.Run("Too many arguments", func(t *testing.T) {
		cmd.SetArgs([]string{"default", "test-pvc", "/tmp/test", "extra"})
		err := cmd.Execute()
		if err == nil {
			t.Error("Expected error when too many arguments are provided")
		}
	})
}
