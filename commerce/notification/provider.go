package notification

import (
	"context"

	"github.com/GoHyperrr/hyperrr/pkg/logger"
)

// Provider defines the interface for sending notifications.
type Provider interface {
	Send(ctx context.Context, n *Notification) error
}

// MockProvider is a simple provider for testing and development.
type MockProvider struct {
	ShouldFail bool
}

func (m *MockProvider) Send(ctx context.Context, n *Notification) error {
	if m.ShouldFail {
		return context.DeadlineExceeded // Simulate a network failure
	}
	logger.Info("MockProvider: Sent notification", "recipient", n.Recipient, "channel", n.Channel, "subject", n.Subject)
	return nil
}
