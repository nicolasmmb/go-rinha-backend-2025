package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/nicolasmmb/go-rinha-backend-2025/internal/core"
	"github.com/nicolasmmb/go-rinha-backend-2025/internal/domain"
)

type PaymentService struct {
	repoPayment core.PaymentRepositoryInterface
	httpClient  *http.Client

	paymentQueue chan domain.Payment
	queueSize    int

	URL_DEFAULT_PROCESSOR  string
	URL_FALLBACK_PROCESSOR string
}

var HackBufferPool = sync.Pool{
	New: func() interface{} {
		return new(bytes.Buffer)
	},
}

func NewPaymentService(paymentRepository core.PaymentRepositoryInterface, URL_DEFAULT_PROCESSOR string, URL_FALLBACK_PROCESSOR string, queueSize int) *PaymentService {
	tr := &http.Transport{
		DialContext: (&net.Dialer{
			Timeout:   500 * time.Millisecond,
			KeepAlive: 30 * time.Second,
		}).DialContext,

		// Reutiliza conexões para diminuir a latência de novas requisições.
		DisableKeepAlives: false,

		// Limita o número total de conexões (ativas + ociosas) por host.
		// Essencial para não sobrecarregar o serviço que você está chamando.
		MaxConnsPerHost: 64,

		// Número de conexões ociosas mantidas no pool para reutilização.
		MaxIdleConns:        64,
		MaxIdleConnsPerHost: 64,

		// Tempo que uma conexão ociosa fica no pool antes de ser fechada.
		IdleConnTimeout: 60 * time.Second,

		// <<< AJUSTE: Desabilitar compressão é bom para serviços internos.
		// A economia de CPU é maior que o ganho de banda na rede local.
		DisableCompression: true,

		// Forçar HTTP/2 pode não ser ideal para POSTs simples e rápidos. HTTP/1.1 é mais previsível aqui.
		ForceAttemptHTTP2: false,
	}

	c := &http.Client{Transport: tr, Timeout: 5 * time.Second}

	return &PaymentService{
		repoPayment:            paymentRepository,
		httpClient:             c,
		URL_DEFAULT_PROCESSOR:  URL_DEFAULT_PROCESSOR,
		URL_FALLBACK_PROCESSOR: URL_FALLBACK_PROCESSOR,
		paymentQueue:           make(chan domain.Payment, queueSize),
		queueSize:              queueSize,
	}
}

func (ps *PaymentService) SendPaymentToQueue(payment *domain.Payment) error {
	select {
	case ps.paymentQueue <- *payment:
		return nil
	default:
		return errors.New("O Galo tá cansado")
	}
}

func (ps *PaymentService) GetPaymentQueue() <-chan domain.Payment {
	return ps.paymentQueue
}

func (ps *PaymentService) sendPaymentRequest(ctx context.Context, payment *domain.Payment, url string) bool {

	buf := HackBufferPool.Get().(*bytes.Buffer)
	buf.Reset()
	defer HackBufferPool.Put(buf)

	if err := json.NewEncoder(buf).Encode(payment); err != nil {
		slog.Warn("falha ao encodar pagamento para JSON", "error", err.Error())
		return false
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, buf)
	if err != nil {
		return false
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := ps.httpClient.Do(req)
	if err != nil {
		return false
	}

	io.Copy(io.Discard, resp.Body)
	defer resp.Body.Close()

	return http.StatusOK == resp.StatusCode
}

func (ps *PaymentService) ProcessPayment(ctx context.Context, p *domain.Payment) (*domain.Payment, error) {
	p.Processor = "default"
	p.RequestedAt = time.Now()

	for i := 0; i < 5; i++ {
		processed := ps.sendPaymentRequest(ctx, p, ps.URL_DEFAULT_PROCESSOR)
		if processed {
			return p, nil
		}
		time.Sleep(5 * time.Millisecond)
	}

	p.Processor = "fallback"
	processed := ps.sendPaymentRequest(ctx, p, ps.URL_FALLBACK_PROCESSOR)
	if processed {
		return p, nil
	}

	return nil, fmt.Errorf("all processors failed")
}

func (ps *PaymentService) GetSummary(ctx context.Context, from, to time.Time) (*domain.Summary, error) {
	dSummaryItems, err := ps.repoPayment.GetSummaryByProcessor(ctx, "default", from, to)
	if err != nil {
		return nil, err
	}

	fSummaryItems, err := ps.repoPayment.GetSummaryByProcessor(ctx, "fallback", from, to)
	if err != nil {
		return nil, err
	}

	return &domain.Summary{Default: *dSummaryItems, Fallback: *fSummaryItems}, nil

}

func (ps *PaymentService) ResetState(ctx context.Context) error {
	return ps.repoPayment.ResetState(ctx)
}

func (ps *PaymentService) SavePayment(ctx context.Context, payment *domain.Payment) error {
	if err := ps.repoPayment.SavePayment(ctx, payment); err != nil {
		return err
	}
	return nil
}
