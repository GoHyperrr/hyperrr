package db

import (
	"os"
	"testing"

	"github.com/GoHyperrr/hyperrr/pkg/config"
)

type User struct {
	ID    uint   `gorm:"primaryKey"`
	Name  string
	Email string `gorm:"unique"`
}

type Order struct {
	ID        uint `gorm:"primaryKey"`
	UserID    uint // Soft relationship, no explicit FK
	Total     float64
}

func TestRepositoryPattern(t *testing.T) {
	dbFile := "repo_test_unique.db"
	if _, err := os.Stat(dbFile); err == nil {
		os.Remove(dbFile)
	}
	defer os.Remove(dbFile)

	cfg := &config.Config{
		DBDriver: "sqlite",
		DBDSN:    dbFile,
	}

	database, err := Connect(cfg)
	if err != nil {
		t.Fatalf("failed to connect: %v", err)
	}

	// Register models from different "modules"
	Register(&User{}, &Order{})

	err = database.AutoMigrateAll()
	if err != nil {
		t.Fatalf("migration failed: %v", err)
	}

	t.Run("Create and Read", func(t *testing.T) {
		user := User{Name: "Alice", Email: "alice@example.com"}
		database.Create(&user)

		var foundUser User
		database.First(&foundUser, user.ID)

		if foundUser.Name != "Alice" {
			t.Errorf("expected Alice, got %s", foundUser.Name)
		}

		order := Order{UserID: user.ID, Total: 100.0}
		database.Create(&order)

		var foundOrder Order
		database.First(&foundOrder, order.ID)

		if foundOrder.UserID != user.ID {
			t.Errorf("expected UserID %d, got %d", user.ID, foundOrder.UserID)
		}
	})
}
