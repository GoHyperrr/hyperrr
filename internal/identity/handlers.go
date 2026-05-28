package identity

import (
	"context"
	"fmt"
	"time"

	"github.com/GoHyperrr/hyperrr/pkg/db"
	"golang.org/x/crypto/bcrypt"
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

// Register creates a new user and actor.
func (m *Module) Register(ctx context.Context, email, password, name string) (*Actor, error) {
	if email == "" || password == "" || name == "" {
		return nil, fmt.Errorf("email, password, and name are required")
	}
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("failed to hash password: %w", err)
	}

	actorID := fmt.Sprintf("act_%d", time.Now().UnixNano())
	actor := Actor{
		ID:   actorID,
		Type: ActorHuman,
		Name: name,
	}

	user := User{
		ID:           fmt.Sprintf("usr_%d", time.Now().UnixNano()),
		Email:        email,
		PasswordHash: string(hashedPassword),
		ActorID:      actorID,
	}

	err = m.database.Transaction(func(tx *db.DB) error {
		if err := tx.Create(&actor).Error; err != nil {
			return err
		}
		if err := tx.Create(&user).Error; err != nil {
			return err
		}
		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to create user: %w", err)
	}

	// Emit event for other modules (like Customer) to react
	m.emit(ctx, "identity.user_created", map[string]any{
		"user_id":  user.ID,
		"actor_id": actor.ID,
		"email":    user.Email,
		"name":     actor.Name,
	})

	return &actor, nil
}

// Login verifies credentials and returns the actor.
func (m *Module) Login(ctx context.Context, email, password string) (*Actor, error) {
	var user User
	if err := m.database.Preload("Actor").First(&user, "email = ?", email).Error; err != nil {
		return nil, fmt.Errorf("invalid credentials")
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		return nil, fmt.Errorf("invalid credentials")
	}

	return &user.Actor, nil
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
