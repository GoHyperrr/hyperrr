package internal

import "testing"

func TestVersion(t *testing.T) {
	if Version != "0.1.0" {
		t.Errorf("Expected version 0.1.0, got %s", Version)
	}
}
