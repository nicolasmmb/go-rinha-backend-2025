package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
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
		Timeout: 5 * time.Second,
		Transport: &http.Transport{
			MaxIdleConns:        10,
			MaxIdleConnsPerHost: 5,
			IdleConnTimeout:     30 * time.Second,
			DisableKeepAlives:   false,
		},
	}}
}

func (s *PaymentService) SendPaymentToQueue(ctx context.Context, payment *domain.Payment) error {
	slog.Info("[SVC:Payment:SavePayemnt] - Saving payment", "correlation_id", payment.CorrelationId)
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
	//
	// provider, _ := s.repoHealthCheck.GetBestProcessingProvider(ctx)

	// switch provider {
	// case "d":
	// 	slog.Info("[SVC:Payment:SavePayemnt] - Using default payment processor", "provider", provider)
	// case "f":
	// 	slog.Info("[SVC:Payment:SavePayemnt] - Using fallback payment processor", "provider", provider)
	// default:
	// 	// re sent
	// 	slog.Error("[SVC:Payment:SavePayemnt] - No valid payment processor available", "provider", provider)
	// }
	return nil
}

func (s *PaymentService) ConsumeMessageFromQueue(ctx context.Context) (*domain.Payment, error) {
	slog.Info("[SVC:Payment:ConsumeMessageFromQueue] - Consuming message from queue")
	payment, err := s.repoPayment.ConsumeMessageFromQueue(ctx)
	if err != nil {
		slog.Error("[SVC:Payment:ConsumeMessageFromQueue] - Failed to consume message from queue", "error", err)
		return nil, err
	}
	slog.Info("[SVC:Payment:ConsumeMessageFromQueue] - Successfully consumed payment", "correlation_id", payment.CorrelationId)

	provider, _ := s.repoHealthCheck.GetBestProcessingProvider(ctx)
	var processorUrl string
	switch provider {
	case "d":
		payment.Processor = "default"
		processorUrl = env.Values.PAYMENT_PROCESSOR_URL_DEFAULT
		slog.Info("[SVC:Payment:ConsumeMessageFromQueue] - Using default payment processor", "provider", provider)
	case "f":
		payment.Processor = "fallback"
		processorUrl = env.Values.PAYMENT_PROCESSOR_URL_FALLBACK
		slog.Info("[SVC:Payment:ConsumeMessageFromQueue] - Using fallback payment processor", "provider", provider)
	default:
		slog.Info("---> [SVC:Payment:ConsumeMessageFromQueue] - No valid payment processor available", "provider", provider)
		// err = s.repoPayment.AddPaymentToQueue(ctx, payment)
		// if err != nil {
		// 	slog.Error("[SVC:Payment:ConsumeMessageFromQueue] - Failed to resend payment to queue", "error", err)
		// 	return nil, err
		// }
	}

	body := map[string]interface{}{
		"correlationId": payment.CorrelationId,
		"amount":        payment.Amount,
		"requestedAt":   payment.RequestedAt.Format(time.RFC3339Nano),
	}

	b, _ := json.Marshal(body)
	req, err := http.NewRequestWithContext(ctx, "POST", processorUrl, bytes.NewReader(b))
	if err != nil {
		log.Printf("[?:Payment:ConsumeMessageFromQueue] Failed to create request: %v, Error: %v, Processor: %s", payment.CorrelationId, err, payment.Processor)
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := s.httpClient.Do(req)
	if err != nil {
		log.Printf("[?:Payment:ConsumeMessageFromQueue] Failed to send request: %v, Error: %v, Processor: %s", payment.CorrelationId, err, payment.Processor)
		s.repoPayment.AddPaymentToQueue(ctx, payment)
		return nil, err
	}
	defer resp.Body.Close()
	// if not between 200 to 300 return error
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		log.Printf("[?:Payment:ConsumeMessageFromQueue] Received non-2xx response: %v, Status Code: %d, Processor: %s", payment.CorrelationId, resp.StatusCode, payment.Processor)
		return nil, fmt.Errorf("received non-2xx response: %d", resp.StatusCode)

	}

	if err := s.repoPayment.SavePayment(ctx, payment); err != nil {
		log.Printf("[?:Payment:ConsumeMessageFromQueue] Failed to save payment: %v, Error: %v, Processor: %s", payment.CorrelationId, err, payment.Processor)
		s.repoPayment.AddPaymentToQueue(ctx, payment)
		return nil, err
	}

	return payment, nil
}
