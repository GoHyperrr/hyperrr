package auth

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/GoHyperrr/hyperrr/pkg/config"
	"github.com/GoHyperrr/hyperrr/pkg/db"
)

func TestAuthStore(t *testing.T) {
	dbFile := "auth_store_test.db"
	defer os.Remove(dbFile)

	cfg := &config.Config{DBDriver: "sqlite", DBDSN: dbFile}
	database, _ := db.Connect(cfg)
	database.AutoMigrate(&Blacklist{}, &RefreshToken{})

	store := NewAuthStore(database)
	ctx := context.Background()

	t.Run("Blacklist", func(t *testing.T) {
		jti := fmt.Sprintf("jti_%d", time.Now().UnixNano())
		err := store.Blacklist(ctx, jti, time.Now().Add(time.Hour))
		if err != nil { t.Error(err) }
		if !store.IsBlacklisted(ctx, jti) { t.Error("failed to blacklist") }
	})

	t.Run("RefreshTokens", func(t *testing.T) {
		token := fmt.Sprintf("token_%d", time.Now().UnixNano())
		err := store.SaveRefreshToken(ctx, &RefreshToken{ID: token, ActorID: "a1", Token: token, ExpiresAt: time.Now().Add(time.Hour)})
		if err != nil { t.Error(err) }
		
		// Test GetRefreshToken success
		got, err := store.GetRefreshToken(ctx, token)
		if err != nil { t.Errorf("GetRefreshToken should not return error: %v", err) }
		if got == nil || got.Token != token { t.Errorf("GetRefreshToken failed: got %v", got) }

		// Test GetRefreshToken error path
		missing, err := store.GetRefreshToken(ctx, "non-existent")
		if err == nil { t.Error("expected error for non-existent token, got nil") }
		if missing != nil { t.Errorf("expected nil token for non-existent, got %v", missing) }

		store.RevokeRefreshToken(ctx, token)
		store.DeleteExpiredTokens(ctx, time.Now())
	})

	t.Run("DeleteExpiredTokens Exhaustive", func(t *testing.T) {
		now := time.Now()
		
		// 1. Expired token
		store.SaveRefreshToken(ctx, &RefreshToken{ID: "expired", ActorID: "u1", Token: "t1", ExpiresAt: now.Add(-time.Hour)})
		
		// 2. Revoked token
		revokedAt := now.Add(-time.Minute)
		store.SaveRefreshToken(ctx, &RefreshToken{ID: "revoked", ActorID: "u1", Token: "t2", ExpiresAt: now.Add(time.Hour), RevokedAt: &revokedAt})
		
		// 3. Valid token
		store.SaveRefreshToken(ctx, &RefreshToken{ID: "valid", ActorID: "u1", Token: "t3", ExpiresAt: now.Add(time.Hour)})

		err := store.DeleteExpiredTokens(ctx, now)
		if err != nil { t.Errorf("DeleteExpiredTokens failed: %v", err) }

		// Verify
		if _, err := store.GetRefreshToken(ctx, "t1"); err == nil { t.Error("expired token should be deleted") }
		if _, err := store.GetRefreshToken(ctx, "t2"); err == nil { t.Error("revoked token should be deleted") }
		if got, err := store.GetRefreshToken(ctx, "t3"); err != nil || got == nil { t.Error("valid token should NOT be deleted") }
	})
}
