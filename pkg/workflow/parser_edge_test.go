package workflow

import (
	"testing"
)

func TestParserEdgeCases(t *testing.T) {
	parser := NewParser()

	t.Run("Empty Name", func(t *testing.T) {
		yamlData := `
steps:
  - id: s1
    uses: u
`
		_, err := parser.Parse([]byte(yamlData))
		if err == nil {
			t.Error("expected error for empty name")
		}
	})

	t.Run("Duplicate Step IDs", func(t *testing.T) {
		yamlData := `
name: dup
steps:
  - id: s1
    uses: u
  - id: s1
    uses: u
`
		_, err := parser.Parse([]byte(yamlData))
		if err == nil {
			t.Error("expected error for duplicate IDs")
		}
	})

	t.Run("Invalid Dependency", func(t *testing.T) {
		yamlData := `
name: inv_dep
steps:
  - id: s1
    uses: u
    depends_on: [ghost]
`
		_, err := parser.Parse([]byte(yamlData))
		if err == nil {
			t.Error("expected error for invalid dependency")
		}
	})
}
