package router

import (
	"context"
	"log"
	"log/slog"
	"net/http"
	"time"

	json "github.com/json-iterator/go"

	"github.com/nicolasmmb/rinha-backend-2025/internal/domain"
	"github.com/nicolasmmb/rinha-backend-2025/internal/model"
	"github.com/nicolasmmb/rinha-backend-2025/internal/service"
)

const (
	ROUTE_PAYMENT_SUMMARY = "GET /payments-summary"
	ROUTE_PAYMENT_GET     = "GET /payments" // Uses query param `correlation_id`
	ROUTE_PAYMENT_SAVE    = "POST /payments"
	ROUTE_HEALTH_CHECK    = "GET /health"
)

type paymentHandler struct {
	Svc *service.PaymentService
}

func NewPaymentHandler(svc *service.PaymentService) *paymentHandler {
	return &paymentHandler{Svc: svc}
}

func (h *paymentHandler) SavePayment(w http.ResponseWriter, r *http.Request) {
	var req model.PaymentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	payment := &domain.Payment{
		CorrelationId: req.CorrelationID,
		Amount:        req.Amount,
		RequestedAt:   time.Now().UTC(),
	}

	// Execute the save operation in a separate goroutine (fire-and-forget)
	// to respond to the client as quickly as possible.
	go func() {
		// It's crucial to recover from potential panics within a goroutine
		// to prevent the entire application from crashing.
		defer func() {
			if r := recover(); r != nil {
				log.Printf("Recovered from panic in SavePayment goroutine: %v", r)
			}
		}()

		// Since this runs asynchronously, we can't return an error to the client.
		// We log it for monitoring and debugging purposes.
		// We use the request's context to propagate cancellation signals.
		if err := h.Svc.SendPaymentToQueue(context.Background(), payment); err != nil {
			log.Printf("Error saving payment asynchronously: %v", err)
		}
	}()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
}

func (h *paymentHandler) GetSummary(w http.ResponseWriter, r *http.Request) {
	fromQuery := r.URL.Query().Get("from")
	var from *time.Time
	if fromQuery != "" {
		f, err := time.Parse(time.RFC3339, fromQuery)
		if err == nil {
			from = &f
		}
	}

	var to *time.Time
	toQuery := r.URL.Query().Get("to")
	if toQuery != "" {
		t, err := time.Parse(time.RFC3339, toQuery)
		if err == nil {
			to = &t
		}
	}
	summary, err := h.Svc.GetSummary(r.Context(), from, to)
	if err != nil {
		http.Error(w, "Failed to get summary", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(summary); err != nil {
		http.Error(w, "Failed to encode summary", http.StatusInternalServerError)
		return
	}
	// slog.Info("Summary retrieved successfully", "count", len(summary), "from", from)

}

func (h *paymentHandler) ResetPayments(w http.ResponseWriter, r *http.Request) {
	if err := h.Svc.ResetState(r.Context()); err != nil {
		http.Error(w, "Failed to reset payments", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func Routes(handler *paymentHandler) *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc(ROUTE_PAYMENT_SAVE, handler.SavePayment)
	mux.HandleFunc(ROUTE_PAYMENT_SUMMARY, handler.GetSummary)
	mux.HandleFunc(ROUTE_PAYMENT_GET, handler.GetPayment)
	mux.HandleFunc(ROUTE_HEALTH_CHECK, handler.HealthCheck)

	return mux

}
func (h *paymentHandler) HealthCheck(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
}

func (h *paymentHandler) GetPayment(w http.ResponseWriter, r *http.Request) {
	tStart := time.Now()
	slog.Info("------------------------------------------------------------------------")
	correlationID := r.URL.Query().Get("correlation_id")
	if correlationID == "" {
		http.Error(w, "Correlation ID is required", http.StatusBadRequest)
		return
	}

	payment, err := h.Svc.GetPaymentByCorrelationID(r.Context(), correlationID)
	if err != nil {
		http.Error(w, "Failed to get payment", http.StatusInternalServerError)
		return
	}

	if payment == nil {
		http.Error(w, "Payment not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(payment)
	took := time.Since(tStart)
	slog.Info("Request processed", "correlation_id", correlationID, "duration", took)
	slog.Info("------------------------------------------------------------------------")
}
