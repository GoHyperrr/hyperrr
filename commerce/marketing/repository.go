package marketing

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

func (r *Repository) SaveCoupon(ctx context.Context, c *Coupon) error {
	return r.db.WithContext(ctx).Save(c).Error
}

func (r *Repository) GetCouponByCode(ctx context.Context, code string) (*Coupon, error) {
	var c Coupon
	err := r.db.WithContext(ctx).First(&c, "code = ? AND active = ?", code, true).Error
	if err != nil {
		return nil, err
	}
	return &c, nil
}

func (r *Repository) SaveLoyaltyPoints(ctx context.Context, lp *LoyaltyPoints) error {
	return r.db.WithContext(ctx).Save(lp).Error
}

func (r *Repository) GetLoyaltyPointsByCustomerID(ctx context.Context, customerID string) (*LoyaltyPoints, error) {
	var lp LoyaltyPoints
	err := r.db.WithContext(ctx).First(&lp, "customer_id = ?", customerID).Error
	if err != nil {
		return nil, err
	}
	return &lp, nil
}
