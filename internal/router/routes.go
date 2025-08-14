package router

import (
	"log/slog"
	"net/http"
	"time"

	json "github.com/json-iterator/go"

	"github.com/nicolasmmb/go-rinha-backend-2025/internal/domain"
	"github.com/nicolasmmb/go-rinha-backend-2025/internal/service"
)

const (
	ROUTE_PAYMENT_SUMMARY = "GET /payments-summary"
	ROUTE_PAYMENT_SAVE    = "POST /payments"
	ROUTE_HEALTH_CHECK    = "GET /health"
	ROUTE_RESET_PAYMENTS  = "GET /reset"
)

type paymentHandler struct {
	Svc *service.PaymentService
}

func NewPaymentHandler(svc *service.PaymentService) *paymentHandler {
	return &paymentHandler{Svc: svc}
}

func (h *paymentHandler) SavePayment(w http.ResponseWriter, r *http.Request) {

	var payment *domain.Payment

	if err := json.NewDecoder(r.Body).Decode(&payment); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	h.Svc.SendPaymentToQueue(payment)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
}

func (h *paymentHandler) GetSummary(w http.ResponseWriter, r *http.Request) {
	fromQuery := r.URL.Query().Get("from")
	toQuery := r.URL.Query().Get("to")

	from, _ := time.Parse(time.RFC3339, fromQuery)
	to, _ := time.Parse(time.RFC3339, toQuery)

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
}

func (h *paymentHandler) ResetPayments(w http.ResponseWriter, r *http.Request) {
	if err := h.Svc.ResetState(r.Context()); err != nil {
		http.Error(w, "Failed to reset payments", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	response := map[string]string{"message": "Payments reset successfully"}
	if err := json.NewEncoder(w).Encode(response); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
		return
	}
	slog.Info("Payments reset successfully")

}

func Routes(handler *paymentHandler) *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc(ROUTE_PAYMENT_SAVE, handler.SavePayment)
	mux.HandleFunc(ROUTE_PAYMENT_SUMMARY, handler.GetSummary)
	mux.HandleFunc(ROUTE_HEALTH_CHECK, handler.HealthCheck)
	mux.HandleFunc(ROUTE_RESET_PAYMENTS, handler.ResetPayments)

	return mux

}
func (h *paymentHandler) HealthCheck(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
}
