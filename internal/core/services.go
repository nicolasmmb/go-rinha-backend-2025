package core

import (
	"context"
	"time"

	"github.com/nicolasmmb/go-rinha-backend-2025/internal/domain"
)

type PaymentRepositoryInterface interface {
	SavePayment(ctx context.Context, payment *domain.Payment) error
	GetSummaryByProcessor(ctx context.Context, typeOfProcessor string, from, to time.Time) (*domain.SummaryItem, error)
	ResetState(ctx context.Context) error
}
