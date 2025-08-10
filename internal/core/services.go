package core

import (
	"context"
	"time"

	"github.com/nicolasmmb/go-rinha-backend-2025/internal/domain"
)

type PaymentRepositoryInterface interface {
	AddPaymentToQueue(ctx context.Context, payment *domain.Payment) error
	GetPaymentFromChannel(ctx context.Context) (*domain.Payment, error)
	GetPaymentByCorrelationID(ctx context.Context, correlationID string) (*domain.Payment, error)
	ConsumeMessageFromQueue(ctx context.Context) (*domain.Payment, error)
	SavePayment(ctx context.Context, payment *domain.Payment) error
	ResetState(ctx context.Context) error
	GetSummary(ctx context.Context, from, to *time.Time) ([]*domain.Payment, error)
}

// , from, to *time.Time
type HealthCheckRepositoryInterface interface {
	HealthCheck(ctx context.Context) error
	Lock(ctx context.Context) error
	Unlock(ctx context.Context) error

	SaveBestProssessingProvider(ctx context.Context, provider string) error
	GetBestProcessingProvider(ctx context.Context) (string, error)
}
