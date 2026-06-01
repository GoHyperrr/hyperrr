package workflow

import (
	"testing"
)

func TestParser(t *testing.T) {
	parser := NewParser()

	t.Run("Valid Workflow", func(t *testing.T) {
		yamlData := `
name: product.enrichment
version: v1
steps:
  - id: fetch
    uses: catalog.fetch
  - id: process
    uses: ai.process
    depends_on: [fetch]
`
		wf, err := parser.Parse([]byte(yamlData))
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if wf.Name != "product.enrichment" {
			t.Errorf("expected name product.enrichment, got %s", wf.Name)
		}
		if len(wf.Steps) != 2 {
			t.Errorf("expected 2 steps, got %d", len(wf.Steps))
		}
	})

	t.Run("Circular Dependency", func(t *testing.T) {
		yamlData := `
name: cycle
steps:
  - id: a
    uses: u
    depends_on: [b]
  - id: b
    uses: u
    depends_on: [a]
`
		_, err := parser.Parse([]byte(yamlData))
		if err == nil {
			t.Fatal("expected error for circular dependency, got nil")
		}
	})

	t.Run("Missing Dependency", func(t *testing.T) {
		yamlData := `
name: missing
steps:
  - id: a
    uses: u
    depends_on: [non-existent]
`
		_, err := parser.Parse([]byte(yamlData))
		if err == nil {
			t.Fatal("expected error for missing dependency, got nil")
		}
	})

	t.Run("Duplicate Step ID", func(t *testing.T) {
		yamlData := `
name: duplicate
steps:
  - id: a
    uses: u1
  - id: a
    uses: u2
`
		_, err := parser.Parse([]byte(yamlData))
		if err == nil {
			t.Fatal("expected error for duplicate step id, got nil")
		}
	})

	t.Run("Missing Name", func(t *testing.T) {
		yamlData := `
steps:
  - id: a
    uses: u
`
		_, err := parser.Parse([]byte(yamlData))
		if err == nil {
			t.Fatal("expected error for missing name, got nil")
		}
	})

	t.Run("No Steps", func(t *testing.T) {
		yamlData := `
name: empty
steps: []
`
		_, err := parser.Parse([]byte(yamlData))
		if err == nil {
			t.Fatal("expected error for no steps, got nil")
		}
	})

	t.Run("Invalid YAML", func(t *testing.T) {
		yamlData := `
name: invalid
  steps: [
`
		_, err := parser.Parse([]byte(yamlData))
		if err == nil {
			t.Fatal("expected error for invalid YAML, got nil")
		}
	})
}
