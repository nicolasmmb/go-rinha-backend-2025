package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
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

	// go func() {
	// 	_ "net/http/pprof"
	// 	log.Println(http.ListenAndServe("localhost:6060", nil))
	// }()

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

	//Initialize Payment Repository and Service
	paymentRepository := redis.NewPaymentsRepository(rds)
	//Initialize Payment Service
	paymentService := service.NewPaymentService(
		paymentRepository,
		env.Values.PAYMENT_PROCESSOR_URL_DEFAULT,
		env.Values.PAYMENT_PROCESSOR_URL_FALLBACK,
		env.Values.PAYMENT_CHAN_SIZE,
	)
	//Initialize Payment Worker
	savePaymentWorker := worker.NewSavePaymentWorker(paymentService, env.Values.WORKER_POOL)
	go savePaymentWorker.RunPaymentProcessor(ctx)

	// Initialize Router and Payment Handler
	paymentHandler := router.NewPaymentHandler(paymentService)
	paymentRoutes := router.Routes(paymentHandler)

	SERVER_HOST := env.Values.SERVER_ADDR + ":" + fmt.Sprint(env.Values.SERVER_PORT)
	server := &http.Server{
		Addr:           SERVER_HOST,
		Handler:        paymentRoutes,
		ReadTimeout:    1 * time.Second,
		WriteTimeout:   1 * time.Second,
		IdleTimeout:    15 * time.Second,
		MaxHeaderBytes: 128 << 10, // 128 KB
	}

	log.Printf("Servidor iniciado em %s", SERVER_HOST)
	libs.GracefulShutdown(server, time.Second*10)
}
