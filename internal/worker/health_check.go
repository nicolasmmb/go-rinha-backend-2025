package worker

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"log/slog"
	"net/http"
	"time"

	"github.com/nicolasmmb/go-rinha-backend-2025/internal/core"
)

const (
	HEALTH_CHECK_INTERVAL = 5500 * time.Millisecond // Interval for health checks
	VALUE_DEFAULT_HEALTH  = "d"
	VALUE_FALLBACK_HEALTH = "f"
)

var ERROR_HEALTH_CHECK = errors.New("health check failed")

// {"failing":false,"minResponseTime":0}%
type HealthStatus struct {
	Failing         bool `json:"failing"`
	MinResponseTime int  `json:"minResponseTime"`
}

type healthCheckWorker struct {
	repo                    core.HealthCheckRepositoryInterface
	SERVICE_HEALTH_DEFAULT  string
	SERVICE_HEALTH_FALLBACK string
}

func NewHealthCheckWorker(repo core.HealthCheckRepositoryInterface, SERVICE_HEALTH_DEFAULT string, SERVICE_HEALTH_FALLBACK string) *healthCheckWorker {
	return &healthCheckWorker{
		repo:                    repo,
		SERVICE_HEALTH_DEFAULT:  SERVICE_HEALTH_DEFAULT,
		SERVICE_HEALTH_FALLBACK: SERVICE_HEALTH_FALLBACK,
	}
}

func (w *healthCheckWorker) Run(ctx context.Context) {
	ticker := time.NewTicker(HEALTH_CHECK_INTERVAL)
	defer ticker.Stop()
	for range ticker.C {
		select {
		case <-ctx.Done():
			slog.Info("Health check worker stopped")
			return
		default:
			w.PerformHealthCheck(ctx)
		}
		log.Println("Updating Health Status")
	}
}

func (w *healthCheckWorker) PerformHealthCheck(ctx context.Context) error {
	slog.Info("Performing health check")
	w.repo.Lock(ctx)
	defer w.repo.Unlock(ctx)

	defaultStatus := w.performHealthCheck(w.SERVICE_HEALTH_DEFAULT)
	if defaultStatus == nil {
		slog.Error("Default health check failed, trying fallback")
	} else if !defaultStatus.Failing {
		return w.repo.SaveBestProssessingProvider(ctx, VALUE_DEFAULT_HEALTH)
	}

	fallbackStatus := w.performHealthCheck(w.SERVICE_HEALTH_FALLBACK)
	if !fallbackStatus.Failing {
		return w.repo.SaveBestProssessingProvider(ctx, VALUE_FALLBACK_HEALTH)
	}

	slog.Error("Both health checks failed, no provider available")
	return ERROR_HEALTH_CHECK
}

func (w *healthCheckWorker) performHealthCheck(url string) *HealthStatus {
	resp, err := http.Get(url)
	if err != nil {
		slog.Error("Health check failed", "error", err)
		return nil
	}
	defer resp.Body.Close()
	var status *HealthStatus

	err = json.NewDecoder(resp.Body).Decode(&status)
	if err != nil {
		slog.Error("Failed to decode health check response", "error", err)
		return nil
	}

	return status
}
