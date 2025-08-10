package redis

import (
	"context"
	"log/slog"
	"time"

	"github.com/nicolasmmb/go-rinha-backend-2025/internal/domain"
	"github.com/redis/go-redis/v9"

	"fmt"

	"github.com/vmihailenco/msgpack/v5"
)

const (
	QUEUE_PAYMENTS           = "queue:payments"
	QUEUE_PAYMENTS_PROCESSED = "queue:payments_processed"
	RD_KEY_TX_PAYMENTS       = "tx:payments"
)

type paymentsRedisRepository struct {
	db             *redis.Client
	QueuedPayments chan *domain.Payment
}

func NewPaymentsRepository(db *redis.Client, QueuedPayments chan *domain.Payment) *paymentsRedisRepository {
	return &paymentsRedisRepository{db: db, QueuedPayments: QueuedPayments}
}

func (r *paymentsRedisRepository) AddPaymentToQueue(ctx context.Context, payment *domain.Payment) error {
	r.QueuedPayments <- payment

	// slog.Info("[RP:Payment:AddPaymentToQueue:00] - Publishing payment to Redis", "correlation_id", payment.CorrelationId)
	// b, err := msgpack.Marshal(payment)
	// if err != nil {
	// 	slog.Error("[RP:Payment:AddPaymentToQueue:01] - Failed to marshal payment", "correlation_id", payment.CorrelationId, "error", err)
	// 	return err
	// }
	// err = r.db.LPush(ctx, QUEUE_PAYMENTS, b).Err()
	// if err != nil {
	// 	slog.Error("[RP:Payment:AddPaymentToQueue:02] - Failed to save payment to Redis", "correlation_id", payment.CorrelationId, "error", err)
	// 	return err
	// }
	return nil
}

func (r *paymentsRedisRepository) GetPaymentFromChannel(ctx context.Context) (*domain.Payment, error) {
	select {
	case payment := <-r.QueuedPayments:
		slog.Info("[RP:Payment:GetPaymentFromChannel] - Retrieved payment from channel", "correlation_id", payment.CorrelationId)
		return payment, nil
	case <-ctx.Done():
		slog.Warn("[RP:Payment:GetPaymentFromChannel] - Context cancelled while waiting for payment")
		return nil, ctx.Err()
	}
}

func (r *paymentsRedisRepository) SavePayment(ctx context.Context, payment *domain.Payment) (err error) {
	b, err := msgpack.Marshal(payment)
	if err != nil {
		slog.Error("[RP:Payment:Save:01] - Failed to marshal payment", "correlation_id", payment.CorrelationId, "error", err)
		return err
	}
	err = r.db.ZAdd(ctx, RD_KEY_TX_PAYMENTS, redis.Z{
		Score:  float64(payment.RequestedAt.UnixNano()),
		Member: b,
	}).Err()

	if err != nil {
		slog.Error("[RP:Payment:Save:02] - Failed to save payment to Redis", "correlation_id", payment.CorrelationId, "error", err)
		return err
	}
	return nil
}

func (r *paymentsRedisRepository) GetSummary(ctx context.Context, from, to *time.Time) ([]*domain.Payment, error) {

	slog.Info("[RP:Payment:GetSummary] - Retrieving payment summary", "from", from, "to", to)
	iter, err := r.db.ZRangeByScore(ctx, RD_KEY_TX_PAYMENTS, &redis.ZRangeBy{
		Min: fmt.Sprintf("%d", from.UnixNano()),
		Max: fmt.Sprintf("%d", to.UnixNano()),
	}).Result()

	if err != nil {
		slog.Error("[RP:Payment:GetSummary:01] - Failed to retrieve payment summary", "error", err)
	}

	var payments []*domain.Payment

	for _, item := range iter {
		var payment domain.Payment

		if err := msgpack.Unmarshal([]byte(item), &payment); err != nil {
			slog.Error("[RP:Payment:GetSummary:02] - Failed to unmarshal payment data", "error", err)
			continue
		}
		payments = append(payments, &payment)
	}

	return payments, nil
}

func (r *paymentsRedisRepository) ResetState(ctx context.Context) error {
	slog.Info("[RP:Payment:ResetState] - Resetting payment state in Redis")
	iter := r.db.Scan(ctx, 0, "*", 0).Iterator()
	for iter.Next(ctx) {
		key := iter.Val()
		if err := r.db.Del(ctx, key).Err(); err != nil {
			slog.Error("[RP:Payment:ResetState:01] - Failed to delete key from Redis", "key", key, "error", err)
			// Decide if you want to continue or return on first error
		} else {
			slog.Info("[RP:Payment:ResetState:02] - Deleted key from Redis", "key", key)
		}
	}
	if err := iter.Err(); err != nil {
		slog.Error("[RP:Payment:ResetState:03] - Error during Redis SCAN", "error", err)
		return err
	}
	return nil
}

func (r *paymentsRedisRepository) GetPaymentByCorrelationID(ctx context.Context, correlationID string) (*domain.Payment, error) {
	// slog.Info("[RP:Payment:GetByCorrelationID] - Retrieving payment by correlation ID", "correlation_id", correlationID)

	data, err := r.db.Get(ctx, correlationID).Bytes()
	if err != nil {
		if err == redis.Nil {
			slog.Warn("[RP:Payment:GetByCorrelationID:01] - Payment not found in Redis", "correlation_id", correlationID)
			return nil, nil
		}
		slog.Error("[RP:Payment:GetByCorrelationID:02] - Failed to get payment from Redis", "correlation_id", correlationID, "error", err)
		return nil, err
	}

	var payment domain.Payment
	if err := msgpack.Unmarshal(data, &payment); err != nil {
		slog.Error("[RP:Payment:GetByCorrelationID:03] - Failed to unmarshal payment data", "correlation_id", correlationID, "error", err)
		return nil, err
	}

	return &payment, nil
}

func (r *paymentsRedisRepository) ConsumeMessageFromQueue(ctx context.Context) (*domain.Payment, error) {
	slog.Info("[RP:Payment:ConsumeMessageFromQueue] - Consuming message from queue")
	data, err := r.db.BRPopLPush(ctx, QUEUE_PAYMENTS, QUEUE_PAYMENTS_PROCESSED, 0).Bytes()
	if err != nil {
		if err == redis.Nil {
			slog.Warn("[RP:Payment:ConsumeMessageFromQueue:01] - No messages in queue")
			return nil, nil
		}

		slog.Error("[RP:Payment:ConsumeMessageFromQueue:02] - Failed to consume message from queue", "error", err)
		return nil, err
	}

	var payment domain.Payment
	if err := msgpack.Unmarshal(data, &payment); err != nil {
		slog.Error("[RP:Payment:ConsumeMessageFromQueue:03] - Failed to unmarshal payment data", "error", err)
		return nil, err
	}

	slog.Info("[RP:Payment:ConsumeMessageFromQueue:04] - Successfully consumed payment", "correlation_id", payment.CorrelationId)
	return &payment, nil
}
