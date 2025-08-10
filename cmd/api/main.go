package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"net/http/pprof"
	"time"

	"github.com/nicolasmmb/go-rinha-backend-2025/internal/config/env"
	"github.com/nicolasmmb/go-rinha-backend-2025/internal/database"
	"github.com/nicolasmmb/go-rinha-backend-2025/internal/domain"
	"github.com/nicolasmmb/go-rinha-backend-2025/internal/repository/redis"
	"github.com/nicolasmmb/go-rinha-backend-2025/internal/router"
	"github.com/nicolasmmb/go-rinha-backend-2025/internal/service"
	"github.com/nicolasmmb/go-rinha-backend-2025/internal/worker"
	"github.com/nicolasmmb/go-rinha-backend-2025/libs"
)

func main() {
	if err := env.Load(); err != nil {
		log.Fatalf("Error loading environment variables: %v", err)
	}
	env.ShowEnvValues()

	rds, err := database.ConnectToRedisClient(env.Values.REDIS_ADDR)
	if err != nil {
		log.Fatalf("Erro ao obter o cliente Redis: %v", err)
	}
	defer database.CloseRedisClient()

	ctx := context.Background()

	// Initialize Health Check Repository
	healthCheckRepo := redis.NewHealthCheckRepository(rds)

	// Initialize Payment Repository and Service
	paymentChannel := make(chan *domain.Payment, env.Values.PAYMENT_CHAN_SIZE)
	paymentRepo := redis.NewPaymentsRepository(rds, paymentChannel)
	paymentSvc := service.NewPaymentService(paymentRepo, healthCheckRepo)
	paymentHandler := router.NewPaymentHandler(paymentSvc)

	// Initialize Health Check Worker
	w := worker.NewHealthCheckWorker(
		healthCheckRepo,
		env.Values.HEALTH_URL_DEFAULT,
		env.Values.HEALTH_URL_FALLBACK,
	)
	go w.Run(ctx)
	// Initialize Save Payment Worker
	savePaymentWorker := worker.NewSavePaymentWorker(
		paymentSvc,
		env.Values.PAYMENT_PROCESSOR_URL_DEFAULT,
		env.Values.PAYMENT_PROCESSOR_URL_FALLBACK,
		env.Values.WORKER_POOL,
		env.Values.PAYMENT_CHAN_SIZE,
	)
	go savePaymentWorker.RunPaymentProcessor(context.Background())

	// Initialize Payment Routes
	time.Sleep(1 * time.Second) // Wait for the worker to start
	paymentRoutes := router.Routes(paymentHandler)

	// Configure Routes
	paymentRoutes.HandleFunc("/debug/pprof/", pprof.Index)
	paymentRoutes.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
	paymentRoutes.HandleFunc("/debug/pprof/profile", pprof.Profile)
	paymentRoutes.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
	paymentRoutes.HandleFunc("/debug/pprof/trace", pprof.Trace)

	SERVER_HOST := env.Values.SERVER_ADDR + ":" + fmt.Sprint(env.Values.SERVER_PORT)
	server := &http.Server{
		Addr:           SERVER_HOST,
		Handler:        paymentRoutes,
		ReadTimeout:    1 * time.Second,
		WriteTimeout:   1 * time.Second,
		IdleTimeout:    30 * time.Second,
		MaxHeaderBytes: 256 << 10, // 256 KB
	}

	log.Printf("Servidor iniciado em %s", SERVER_HOST)
	libs.GracefulShutdown(server, time.Second*10)
}
