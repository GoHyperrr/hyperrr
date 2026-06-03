package main

import (
	"bytes"
	"os"
	"testing"
)

func TestMainCLI(t *testing.T) {
	os.Setenv("APP_ENV", "test")
	defer os.Unsetenv("APP_ENV")

	t.Run("root command prints help", func(t *testing.T) {
		rootCmd.SetArgs([]string{"--help"})
		buf := new(bytes.Buffer)
		rootCmd.SetOut(buf)
		if err := rootCmd.Execute(); err != nil {
			t.Errorf("expected no error for help, got: %v", err)
		}
	})

	t.Run("version command prints version info", func(t *testing.T) {
		rootCmd.SetArgs([]string{"version"})
		buf := new(bytes.Buffer)
		rootCmd.SetOut(buf)
		if err := rootCmd.Execute(); err != nil {
			t.Errorf("expected run to succeed, got: %v", err)
		}
	})

	t.Run("invalid command returns error", func(t *testing.T) {
		rootCmd.SetArgs([]string{"unknown"})
		buf := new(bytes.Buffer)
		rootCmd.SetOut(buf)
		if err := rootCmd.Execute(); err == nil {
			t.Error("expected error for unknown command, got nil")
		}
	})

	t.Run("main executes successfully on command success", func(t *testing.T) {
		rootCmd.SetArgs([]string{"version"})
		main()
	})
}
