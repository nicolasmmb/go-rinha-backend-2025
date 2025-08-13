package redis

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/nicolasmmb/go-rinha-backend-2025/internal/core"
	"github.com/nicolasmmb/go-rinha-backend-2025/internal/domain"
	"github.com/redis/go-redis/v9"
)

const (
	RD_KEY_TX_PAYMENTS_PAYLOAD  = "tx:payload:%s"
	RD_KEY_TX_PAYMENTS_TIMELINE = "tx:timeline:%s"
)

type paymentsRedisRepository struct {
	db *redis.Client
}

func NewPaymentsRepository(db *redis.Client) core.PaymentRepositoryInterface {
	return &paymentsRedisRepository{db: db}
}

func (r *paymentsRedisRepository) SavePayment(ctx context.Context, payment *domain.Payment) (err error) {

	pipeline := r.db.Pipeline()

	pipeline.ZAdd(
		ctx,
		fmt.Sprintf(RD_KEY_TX_PAYMENTS_TIMELINE, payment.Processor),
		redis.Z{
			Score:  float64(payment.RequestedAt.UnixNano()),
			Member: payment.CorrelationId,
		},
	)

	pipeline.HSet(
		ctx,
		fmt.Sprintf(RD_KEY_TX_PAYMENTS_PAYLOAD, payment.Processor),
		payment.CorrelationId, payment.Amount,
	)

	if _, err := pipeline.Exec(ctx); err != nil {
		return err
	}

	return nil
}

func (r *paymentsRedisRepository) GetSummaryByProcessor(ctx context.Context, typeOfProcessor string, from, to time.Time) (*domain.SummaryItem, error) {

	pIds, err := r.db.ZRangeByScore(ctx,
		fmt.Sprintf(RD_KEY_TX_PAYMENTS_TIMELINE, typeOfProcessor),
		&redis.ZRangeBy{
			Min: strconv.FormatInt(from.UnixNano(), 10),
			Max: strconv.FormatInt(to.UnixNano(), 10),
		},
	).Result()

	if err != nil {
		return nil, err
	}

	if len(pIds) == 0 {
		return &domain.SummaryItem{}, nil
	}

	values, err := r.db.HMGet(ctx, fmt.Sprintf(RD_KEY_TX_PAYMENTS_PAYLOAD, typeOfProcessor), pIds...).Result()

	if err != nil {
		return nil, err
	}

	result := &domain.SummaryItem{}
	for _, value := range values {
		if valueStr, ok := value.(string); ok {
			if amount, err := strconv.ParseFloat(valueStr, 64); err == nil {
				result.TotalAmount += amount
				result.TotalRequests++
			}
		}
	}

	return result, nil
}

func (r *paymentsRedisRepository) ResetState(ctx context.Context) error {
	return r.db.FlushDB(ctx).Err()
}
