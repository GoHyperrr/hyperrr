package main

import (
	"fmt"
	"os"
	"testing"
)

func TestMainFunc(t *testing.T) {
	os.Setenv("APP_ENV", "test")
	defer os.Unsetenv("APP_ENV")

	t.Run("main success", func(t *testing.T) {
		oldExit := osExit
		defer func() { osExit = oldExit }()
		osExit = func(err error) {}
		
		oldAppRun := appRun
		defer func() { appRun = oldAppRun }()
		appRun = func() error { return nil }
		
		main()
	})

	t.Run("main failure", func(t *testing.T) {
		oldExit := osExit
		called := false
		defer func() { osExit = oldExit }()
		osExit = func(err error) { called = true }
		
		oldAppRun := appRun
		defer func() { appRun = oldAppRun }()
		appRun = func() error { return fmt.Errorf("forced error") }
		
		main()
		if !called {
			t.Error("expected osExit to be called on error")
		}
	})
}
