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
	"github.com/nicolasmmb/go-rinha-backend-2025/internal/repository/redis"
	"github.com/nicolasmmb/go-rinha-backend-2025/internal/router"
	"github.com/nicolasmmb/go-rinha-backend-2025/internal/service"
	"github.com/nicolasmmb/go-rinha-backend-2025/internal/worker"
	"github.com/nicolasmmb/go-rinha-backend-2025/libs"
)

func main() {
	env.Load()
	env.ShowEnvValues()

	rds, err := database.ConnectToRedisClient(env.Values.REDIS_ADDR)
	if err != nil {
		log.Fatalf("Erro ao obter o cliente Redis: %v", err)
	}
	defer database.CloseRedisClient()

	// Initialize Health Check Repository
	healthCheckRepo := redis.NewHealthCheckRepository(rds)

	// Initialize Payment Repository and Service
	paymentRepo := redis.NewPaymentsRepository(rds)
	paymentSvc := service.NewPaymentService(paymentRepo, healthCheckRepo)
	paymentHandler := router.NewPaymentHandler(paymentSvc)

	// Initialize Health Check Worker
	w := worker.NewHealthCheckWorker(healthCheckRepo, env.Values.HEALTH_URL_DEFAULT, env.Values.HEALTH_URL_FALLBACK)
	go w.Run(context.Background())
	// Initialize Save Payment Worker
	savePaymentWorker := worker.NewSavePaymentWorker(paymentSvc, env.Values.PAYMENT_PROCESSOR_URL_DEFAULT, env.Values.PAYMENT_PROCESSOR_URL_FALLBACK, env.Values.WORKER_POOL, 2000)
	go savePaymentWorker.RunPaymentProcessor(context.Background())

	// Initialize Payment Routes
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
