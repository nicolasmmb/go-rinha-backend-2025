package redis

import (
	"context"
	"log/slog"
	"sync/atomic"
	"time"

	"github.com/redis/go-redis/v9"
)

type healthCheckRedisRepository struct {
	db      *redis.Client
	Healthy atomic.Value
}

func NewHealthCheckRepository(db *redis.Client) *healthCheckRedisRepository {
	return &healthCheckRedisRepository{db: db}

}

func (r *healthCheckRedisRepository) HealthCheck(ctx context.Context) error {
	_, err := r.db.Ping(ctx).Result()
	if err != nil {
		slog.Error("Redis health check failed", "error", err)
		return err
	}
	slog.Info("Redis health check successful")
	return nil
}
func (r *healthCheckRedisRepository) Lock(ctx context.Context) error {
	slog.Info("Locking health check repository")
	return r.db.SetNX(ctx, "health_check_locked", true, 5*time.Second).Err()
}
func (r *healthCheckRedisRepository) Unlock(ctx context.Context) error {
	slog.Info("Unlocking health check repository")
	return r.db.Set(ctx, "health_check_locked", false, 5*time.Second).Err()

}
func (r *healthCheckRedisRepository) ResetState(ctx context.Context) error {
	slog.Info("[RP:HealthCheck:ResetState] - Resetting health check state in Redis")
	return nil
}

func (r *healthCheckRedisRepository) SaveBestProssessingProvider(ctx context.Context, provider string) error {
	slog.Info("[RP:HealthCheck:SaveBestProcessingProvider] - Saving best processing provider", "provider", provider)
	r.Healthy.Store(provider)
	return r.db.Set(ctx, "best_processing_provider", provider, 0).Err()
}
func (r *healthCheckRedisRepository) GetBestProcessingProvider(ctx context.Context) (string, error) {
	slog.Info("[RP:HealthCheck:GetBestProcessingProvider] - Retrieving best processing provider")
	// try to get from atomic
	if provider, ok := r.Healthy.Load().(string); ok {
		return provider, nil
	}
	provider, err := r.db.Get(ctx, "best_processing_provider").Result()
	if err != nil {
		slog.Info("[RP:HealthCheck:GetBestProcessingProvider] - Failed to retrieve best processing provider", "error", err)
		// If error, return default provider
		return "d", err
	}
	return provider, nil
}
