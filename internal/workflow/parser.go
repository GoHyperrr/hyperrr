package workflow

import (
	"fmt"

	"gopkg.in/yaml.v3"
)

// Parser parses and validates workflow definitions.
type Parser struct{}

// NewParser creates a new Parser.
func NewParser() *Parser {
	return &Parser{}
}

// Parse parses YAML bytes into a Workflow and validates its DAG.
func (p *Parser) Parse(data []byte) (*Workflow, error) {
	var wf Workflow
	if err := yaml.Unmarshal(data, &wf); err != nil {
		return nil, fmt.Errorf("failed to unmarshal workflow: %w", err)
	}

	if err := p.Validate(&wf); err != nil {
		return nil, err
	}

	return &wf, nil
}

// Validate ensures the workflow is a valid DAG.
func (p *Parser) Validate(wf *Workflow) error {
	if wf.Name == "" {
		return fmt.Errorf("workflow name is required")
	}

	if len(wf.Steps) == 0 {
		return fmt.Errorf("at least one step is required")
	}

	stepMap := make(map[string]Step)
	for _, step := range wf.Steps {
		if _, exists := stepMap[step.ID]; exists {
			return fmt.Errorf("duplicate step id: %s", step.ID)
		}
		stepMap[step.ID] = step
	}

	// Build adjacency list and check for cycles/missing dependencies
	adj := make(map[string][]string)
	for _, step := range wf.Steps {
		for _, dep := range step.DependsOn {
			if _, exists := stepMap[dep]; !exists {
				return fmt.Errorf("step %s depends on non-existent step %s", step.ID, dep)
			}
			adj[dep] = append(adj[dep], step.ID)
		}
	}

	if p.hasCycle(wf.Steps, adj) {
		return fmt.Errorf("workflow contains a circular dependency")
	}

	return nil
}

func (p *Parser) hasCycle(steps []Step, adj map[string][]string) bool {
	visited := make(map[string]int) // 0: unvisited, 1: visiting, 2: visited

	var visit func(id string) bool
	visit = func(id string) bool {
		visited[id] = 1
		for _, neighbor := range adj[id] {
			if visited[neighbor] == 1 {
				return true
			}
			if visited[neighbor] == 0 {
				if visit(neighbor) {
					return true
				}
			}
		}
		visited[id] = 2
		return false
	}

	for _, step := range steps {
		if visited[step.ID] == 0 {
			if visit(step.ID) {
				return true
			}
		}
	}

	return false
}
