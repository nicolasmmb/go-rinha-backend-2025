package domain

import (
	"time"

	"github.com/google/uuid"
)

type Payment struct {
	CorrelationId string // Tem que ser um UUID valido no momento sem validação
	Amount        float64
	Processor     string // "default" ou "fallback"
	RequestedAt   time.Time
}

func (p *Payment) ValidateCorrelationId() bool {
	_, err := uuid.Parse(p.CorrelationId)
	return err == nil
}
