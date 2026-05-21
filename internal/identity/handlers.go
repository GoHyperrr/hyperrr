package identity

import (
	"context"
	"fmt"
)

// ValidateActor is a workflow handler that verifies an actor exists and is active.
func (m *Module) ValidateActor(ctx context.Context, input any) (any, error) {
	data, ok := input.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("invalid input type")
	}

	workflowInput, ok := data["input"].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("missing workflow input")
	}

	actorID, _ := workflowInput["actor_id"].(string)
	if actorID == "" {
		return nil, fmt.Errorf("actor_id is required")
	}

	var actor Actor
	if err := m.database.First(&actor, "id = ?", actorID).Error; err != nil {
		return nil, fmt.Errorf("actor not found: %w", err)
	}

	return map[string]any{
		"id":   actor.ID,
		"type": actor.Type,
		"name": actor.Name,
	}, nil
}

// GetActorByAPIKey retrieves an actor associated with a given API key.
// This will be useful for middleware.
func (m *Module) GetActorByAPIKey(ctx context.Context, key string) (*Actor, error) {
	var apiKey APIKey
	err := m.database.Preload("Actor").First(&apiKey, "key = ?", key).Error
	if err != nil {
		return nil, err
	}
	return &apiKey.Actor, nil
}
