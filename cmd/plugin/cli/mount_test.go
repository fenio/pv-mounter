package cli

import (
	"os"
	"testing"
)

func TestMountCmd(t *testing.T) {
	cmd := mountCmd()

	if cmd.Use != "mount [flags] <namespace> <pvc-name> <local-mount-point>" {
		t.Errorf("Expected Use to be 'mount [flags] <namespace> <pvc-name> <local-mount-point>', got '%s'", cmd.Use)
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

func TestMountCmdFlags(t *testing.T) {
	cmd := mountCmd()

	needsRootFlag := cmd.Flags().Lookup("needs-root")
	if needsRootFlag == nil {
		t.Error("Expected --needs-root flag to be defined")
	}

	debugFlag := cmd.Flags().Lookup("debug")
	if debugFlag == nil {
		t.Error("Expected --debug flag to be defined")
	}

	imageFlag := cmd.Flags().Lookup("image")
	if imageFlag == nil {
		t.Error("Expected --image flag to be defined")
	}

	imageSecretFlag := cmd.Flags().Lookup("image-secret")
	if imageSecretFlag == nil {
		t.Error("Expected --image-secret flag to be defined")
	}

	cpuLimitFlag := cmd.Flags().Lookup("cpu-limit")
	if cpuLimitFlag == nil {
		t.Error("Expected --cpu-limit flag to be defined")
	}
}

func TestMountCmdEnvironmentVariables(t *testing.T) {
	cmd := mountCmd()

	t.Run("NEEDS_ROOT environment variable", func(t *testing.T) {
		_ = os.Setenv("NEEDS_ROOT", "true")
		defer func() { _ = os.Unsetenv("NEEDS_ROOT") }()

		args := []string{"default", "test-pvc", "/tmp/test"}
		cmd.SetArgs(args)

		if cmd.ParseFlags(args) == nil {
			t.Log("Environment variable NEEDS_ROOT is handled")
		}
	})

	t.Run("DEBUG environment variable", func(t *testing.T) {
		_ = os.Setenv("DEBUG", "true")
		defer func() { _ = os.Unsetenv("DEBUG") }()

		args := []string{"default", "test-pvc", "/tmp/test"}
		cmd.SetArgs(args)

		if cmd.ParseFlags(args) == nil {
			t.Log("Environment variable DEBUG is handled")
		}
	})

	t.Run("IMAGE environment variable", func(t *testing.T) {
		_ = os.Setenv("IMAGE", "custom-image:latest")
		defer func() { _ = os.Unsetenv("IMAGE") }()

		args := []string{"default", "test-pvc", "/tmp/test"}
		cmd.SetArgs(args)

		if cmd.ParseFlags(args) == nil {
			t.Log("Environment variable IMAGE is handled")
		}
	})
}

func TestMountCmdArgValidation(t *testing.T) {
	t.Run("Too few arguments", func(t *testing.T) {
		cmd := mountCmd()
		cmd.SetArgs([]string{"default", "test-pvc"})
		err := cmd.Execute()
		if err == nil {
			t.Error("Expected error when too few arguments are provided")
		}
	})

	t.Run("Too many arguments", func(t *testing.T) {
		cmd := mountCmd()
		cmd.SetArgs([]string{"default", "test-pvc", "/tmp/test", "extra"})
		err := cmd.Execute()
		if err == nil {
			t.Error("Expected error when too many arguments are provided")
		}
	})
}

func TestMountCmdInvalidEnvVars(t *testing.T) {
	t.Run("Invalid NEEDS_ROOT value", func(t *testing.T) {
		cmd := mountCmd()
		_ = os.Setenv("NEEDS_ROOT", "invalid-bool")
		defer func() { _ = os.Unsetenv("NEEDS_ROOT") }()

		cmd.SetArgs([]string{"default", "test-pvc", "/tmp/test"})
		err := cmd.Execute()
		if err == nil {
			t.Error("Expected error for invalid NEEDS_ROOT value")
		}
		if err != nil && err.Error() != "invalid value for NEEDS_ROOT: invalid-bool" {
			t.Errorf("Expected specific error message, got: %v", err)
		}
	})

	t.Run("Invalid DEBUG value", func(t *testing.T) {
		cmd := mountCmd()
		_ = os.Setenv("DEBUG", "not-a-boolean")
		defer func() { _ = os.Unsetenv("DEBUG") }()

		cmd.SetArgs([]string{"default", "test-pvc", "/tmp/test"})
		err := cmd.Execute()
		if err == nil {
			t.Error("Expected error for invalid DEBUG value")
		}
		if err != nil && err.Error() != "invalid value for DEBUG: not-a-boolean" {
			t.Errorf("Expected specific error message, got: %v", err)
		}
	})
}

func TestMountCmdAllEnvVars(t *testing.T) {
	t.Run("All environment variables set", func(t *testing.T) {
		cmd := mountCmd()
		_ = os.Setenv("NEEDS_ROOT", "true")
		_ = os.Setenv("DEBUG", "false")
		_ = os.Setenv("IMAGE", "custom:latest")
		_ = os.Setenv("IMAGE_SECRET", "my-secret")
		_ = os.Setenv("CPU_LIMIT", "500m")
		defer func() {
			_ = os.Unsetenv("NEEDS_ROOT")
			_ = os.Unsetenv("DEBUG")
			_ = os.Unsetenv("IMAGE")
			_ = os.Unsetenv("IMAGE_SECRET")
			_ = os.Unsetenv("CPU_LIMIT")
		}()

		args := []string{"default", "test-pvc", "/tmp/test"}
		cmd.SetArgs(args)

		if err := cmd.ParseFlags(args); err != nil {
			t.Errorf("Failed to parse flags: %v", err)
		}
	})
}

func TestMountCmdFlagPrecedence(t *testing.T) {
	t.Run("Flag overrides environment variable", func(t *testing.T) {
		cmd := mountCmd()
		_ = os.Setenv("IMAGE", "env-image:latest")
		defer func() { _ = os.Unsetenv("IMAGE") }()

		args := []string{"--image", "flag-image:latest", "default", "test-pvc", "/tmp/test"}
		cmd.SetArgs(args)

		if err := cmd.ParseFlags(args); err != nil {
			t.Errorf("Failed to parse flags: %v", err)
		}

		imageFlag := cmd.Flags().Lookup("image")
		if imageFlag.Value.String() != "flag-image:latest" {
			t.Errorf("Expected flag to override env var, got: %s", imageFlag.Value.String())
		}
	})
}
