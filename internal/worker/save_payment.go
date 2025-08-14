package worker

import (
	"context"

	"github.com/nicolasmmb/go-rinha-backend-2025/internal/domain"
	"github.com/nicolasmmb/go-rinha-backend-2025/internal/service"
)

type savePaymentWorker struct {
	svc     *service.PaymentService
	WORKERS int
}

func NewSavePaymentWorker(svc *service.PaymentService, WORKERS int) *savePaymentWorker {
	return &savePaymentWorker{svc: svc, WORKERS: WORKERS}
}

func (w *savePaymentWorker) RunPaymentProcessor(ctx context.Context) {
	queue := w.svc.GetPaymentQueue()
	for i := 0; i < w.WORKERS; i++ {
		go w.processPayments(ctx, queue)
	}
}

func (w *savePaymentWorker) processPayments(ctx context.Context, queue <-chan domain.Payment) {

	var payment domain.Payment
	var p *domain.Payment
	var err error

	for payment = range queue {
		p, err = w.svc.ProcessPayment(ctx, &payment)
		if err != nil {
			continue
		}
		w.svc.SavePayment(ctx, p)
	}
}
