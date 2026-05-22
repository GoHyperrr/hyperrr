package support

import (
	"context"

	"github.com/GoHyperrr/hyperrr/pkg/db"
)

type Repository struct {
	db *db.DB
}

func NewRepository(database *db.DB) *Repository {
	return &Repository{db: database}
}

func (r *Repository) SaveTicket(ctx context.Context, t *Ticket) error {
	return r.db.WithContext(ctx).Save(t).Error
}

func (r *Repository) SaveMessage(ctx context.Context, m *Message) error {
	return r.db.WithContext(ctx).Save(m).Error
}

func (r *Repository) GetTicketByID(ctx context.Context, id string) (*Ticket, error) {
	var t Ticket
	err := r.db.WithContext(ctx).Preload("Messages").First(&t, "id = ?", id).Error
	if err != nil {
		return nil, err
	}
	return &t, nil
}

func (r *Repository) ListTicketsByCustomerID(ctx context.Context, customerID string) ([]*Ticket, error) {
	var tickets []*Ticket
	err := r.db.WithContext(ctx).Preload("Messages").Where("customer_id = ?", customerID).Find(&tickets).Error
	return tickets, err
}
