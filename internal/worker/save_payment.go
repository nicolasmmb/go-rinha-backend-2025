package worker

import (
	"context"
	"log/slog"

	"github.com/nicolasmmb/go-rinha-backend-2025/internal/domain"
	"github.com/nicolasmmb/go-rinha-backend-2025/internal/service"
)

type savePaymentWorker struct {
	svc     *service.PaymentService
	WORKERS int

	PAYMENT_PROCESSOR_URL_DEFAULT  string
	PAYMENT_PROCESSOR_URL_FALLBACK string
	QueueItems                     chan []*domain.Payment
}

func NewSavePaymentWorker(svc *service.PaymentService, PAYMENT_PROCESSOR_URL_DEFAULT string, PAYMENT_PROCESSOR_URL_FALLBACK string, WORKERS int, queueLen int) *savePaymentWorker {
	return &savePaymentWorker{
		svc:                            svc,
		WORKERS:                        WORKERS,
		QueueItems:                     make(chan []*domain.Payment, queueLen),
		PAYMENT_PROCESSOR_URL_DEFAULT:  PAYMENT_PROCESSOR_URL_DEFAULT,
		PAYMENT_PROCESSOR_URL_FALLBACK: PAYMENT_PROCESSOR_URL_FALLBACK,
	}
}

func (w *savePaymentWorker) RunPaymentProcessor(ctx context.Context) {
	for i := 0; i < w.WORKERS; i++ {
		go w.processPayments(ctx)
	}
}

func (w *savePaymentWorker) processPayments(ctx context.Context) {
	for {
		payment, err := w.svc.ConsumeMessageFromQueue(ctx)
		if err != nil {
			slog.Warn("[Worker:SavePayment:processPayments] - Failed to consume payment from queue", "error", err)
			continue
		}
		w.svc.SavePayment(ctx, payment)
	}

}
