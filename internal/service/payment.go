package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/nicolasmmb/go-rinha-backend-2025/internal/config/env"
	"github.com/nicolasmmb/go-rinha-backend-2025/internal/core"
	"github.com/nicolasmmb/go-rinha-backend-2025/internal/domain"
)

type PaymentService struct {
	repoPayment     core.PaymentRepositoryInterface
	repoHealthCheck core.HealthCheckRepositoryInterface
	httpClient      *http.Client
}

func NewPaymentService(paymentRepository core.PaymentRepositoryInterface, healthCheckRepository core.HealthCheckRepositoryInterface) *PaymentService {
	return &PaymentService{repoPayment: paymentRepository, repoHealthCheck: healthCheckRepository, httpClient: &http.Client{
		Timeout:   2 * time.Second,

	}}
}

func (s *PaymentService) SendPaymentToQueue(ctx context.Context, payment *domain.Payment) error {
	slog.Info("[SVC:Payment:SendPaymentToQueue] - Saving payment", "correlation_id", payment.CorrelationId)
	return s.repoPayment.AddPaymentToQueue(ctx, payment)
}

func (s *PaymentService) GetSummary(ctx context.Context, from, to *time.Time) (*domain.Summary, error) {
	payments, err := s.repoPayment.GetSummary(ctx, from, to)
	if err != nil {
		slog.Error("[SVC:Payment:GetSummary] - Failed to get payment summary",
			"error", err, "from", from, "to", to)
		return nil, err
	}

	var defaultCount, fallbackCount int
	var defaultAmount, fallbackAmount float64

	for _, payment := range payments {
		if payment.Processor == "default" {
			defaultCount++
			defaultAmount += payment.Amount
		}
		if payment.Processor == "fallback" {
			fallbackCount++
			fallbackAmount += payment.Amount
		}
	}
	x := &domain.Summary{
		Default: domain.SummaryItem{
			TotalRequests: defaultCount,
			TotalAmount:   defaultAmount,
		},
		Fallback: domain.SummaryItem{
			TotalRequests: fallbackCount,
			TotalAmount:   fallbackAmount,
		},
	}

	return x, err
}

func (s *PaymentService) ResetState(ctx context.Context) error {
	return s.repoPayment.ResetState(ctx)
}
func (s *PaymentService) GetPaymentByCorrelationID(ctx context.Context, correlationID string) (*domain.Payment, error) {
	slog.Info("Retrieving payment by correlation ID", "correlation_id", correlationID)
	payment, err := s.repoPayment.GetPaymentByCorrelationID(ctx, correlationID)
	if err != nil {
		slog.Error("Failed to retrieve payment", "correlation_id", correlationID, "error", err)
		return nil, err
	}
	return payment, nil
}

func (s *PaymentService) SavePayment(ctx context.Context, payment *domain.Payment) error { // Renamed from SavePayemnt
	slog.Info("[SVC:Payment:SavePayemnt] - Saving payment", "correlation_id", payment.CorrelationId)

	return nil
}

func (s *PaymentService) ConsumeMessageFromQueue(ctx context.Context) (*domain.Payment, error) {
	slog.Info("[SVC:Payment:ConsumeMessageFromQueue:01] - Consuming message from queue")
	payment, err := s.repoPayment.GetPaymentFromChannel(ctx)
	if err != nil {
		slog.Error("[SVC:Payment:ConsumeMessageFromQueue:02] - Failed to consume message from queue", "error", err)
		return nil, err
	}
	slog.Info("[SVC:Payment:ConsumeMessageFromQueue:03] - Successfully consumed payment", "correlation_id", payment.CorrelationId)

	provider, _ := s.repoHealthCheck.GetBestProcessingProvider(ctx)
	var processorUrl string
	slog.Info("[SVC:Payment:ConsumeMessageFromQueue:04-0] - Best processing provider", "provider", provider)
	switch provider {
	case "d":
		payment.Processor = "default"
		processorUrl = env.Values.PAYMENT_PROCESSOR_URL_DEFAULT
		slog.Info("[SVC:Payment:ConsumeMessageFromQueue:04-1] - Using default payment processor", "provider", provider)
	case "f":
		payment.Processor = "fallback"
		processorUrl = env.Values.PAYMENT_PROCESSOR_URL_FALLBACK
		slog.Info("[SVC:Payment:ConsumeMessageFromQueue:04-2] - Using fallback payment processor", "provider", provider)
	default:
		slog.Info("---> [SVC:Payment:ConsumeMessageFromQueue:05] - No valid payment processor available", "provider", provider)
		return nil, fmt.Errorf("no valid payment processor available: %s", provider)
	}
	t := time.Now().UTC().Format(time.RFC3339)
	slog.Info("[SVC:Payment:ConsumeMessageFromQueue:06] - TIME FORMAT", "time", t, "correlation_id", payment.CorrelationId)
	body := map[string]interface{}{
		"correlationId": payment.CorrelationId,
		"amount":        payment.Amount,
		"requestedAt":   t,
	}

	b, _ := json.Marshal(body)
	req, err := http.NewRequestWithContext(ctx, "POST", processorUrl, bytes.NewReader(b))
	if err != nil {
		slog.Error("[SVC:Payment:ConsumeMessageFromQueue:07] - Failed to create HTTP request", "error", err)
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	resp, err := s.httpClient.Do(req)
	if err != nil {

		if payment.Processor == "default" {
			slog.Error("[SVC:Payment:ConsumeMessageFromQueue:08] - Failed to process payment with default processor", "error", err)
			s.repoHealthCheck.SaveBestProssessingProvider(ctx, "f") // Save fallback
		} else {
			slog.Error("[SVC:Payment:ConsumeMessageFromQueue:09] - Failed to process payment with fallback processor", "error", err)
			s.repoHealthCheck.SaveBestProssessingProvider(ctx, "d") // Save default
		}
		slog.Info("[SVC:Payment:ConsumeMessageFromQueue:10] - Re-adding payment to queue", "correlation_id", payment.CorrelationId)
		s.repoPayment.AddPaymentToQueue(ctx, payment)
		return nil, err
	}
	defer resp.Body.Close()
	// if not between 200 to 300 return error
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		slog.Error("[SVC:Payment:ConsumeMessageFromQueue:11] - Received non-2xx response from payment processor", "status_code", resp.StatusCode, "correlation_id", payment.CorrelationId)
		// s.repoHealthCheck.SaveBestProssessingProvider(ctx, "f") // Save fallback
		return nil, fmt.Errorf("received non-2xx response: %d", resp.StatusCode)

	}

	if err := s.repoPayment.SavePayment(ctx, payment); err != nil {
		slog.Error("[SVC:Payment:ConsumeMessageFromQueue:12] - Failed to save payment", "error", err, "correlation_id", payment.CorrelationId)
		s.repoPayment.AddPaymentToQueue(ctx, payment)
		return nil, err
	}
	slog.Info("[SVC:Payment:ConsumeMessageFromQueue:13] - Payment processed successfully", "correlation_id", payment.CorrelationId)
	return payment, nil
}
