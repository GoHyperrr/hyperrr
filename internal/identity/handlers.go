package identity

import (
	"context"
	"fmt"

	"github.com/GoHyperrr/hyperrr/pkg/db"
	"github.com/GoHyperrr/hyperrr/pkg/utils"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

// ValidateActor is a workflow handler that verifies an actor exists and is active.
func (m *Module) ValidateActor(ctx context.Context, input any) (any, error) {
	data, ok := input.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("invalid input type")
	}

	workflowInput := utils.GetMap(data, "input")
	if workflowInput == nil {
		return nil, fmt.Errorf("missing workflow input")
	}

	actorID := utils.GetString(workflowInput, KeyActorID)
	if actorID == "" {
		return nil, fmt.Errorf("actor_id is required")
	}

	var actor Actor
	if err := m.database.First(&actor, "id = ?", actorID).Error; err != nil {
		return nil, fmt.Errorf("actor not found: %w", err)
	}

	return map[string]any{
		KeyID:   actor.ID,
		KeyType: actor.Type,
		KeyName: actor.Name,
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

	actorID := "act_" + uuid.New().String()
	actor := Actor{
		ID:   actorID,
		Type: ActorHuman,
		Name: name,
	}

	user := User{
		ID:           "usr_" + uuid.New().String(),
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
	m.emit(ctx, EventUserCreated, map[string]any{
		KeyUserID:  user.ID,
		KeyActorID: actor.ID,
		KeyEmail:   user.Email,
		KeyName:    actor.Name,
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
		return nil, fmt.Errorf("invalid API key")
	}
	return &apiKey.Actor, nil
}
