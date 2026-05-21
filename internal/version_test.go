package internal

import "testing"

func TestVersion(t *testing.T) {
	if Version != "0.0.1" {
		t.Errorf("Expected version 0.0.1, got %s", Version)
	}
}
