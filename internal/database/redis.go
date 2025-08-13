package database

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

var (
	client *redis.Client
	once   sync.Once

	err error
)

func ConnectToRedisClient(addr string) (*redis.Client, error) {
	once.Do(func() {
		// Esta função anônima será executada apenas uma vez.
		log.Println("⚙️  Iniciando conexão com o Redis...")

		if addr == "" {
			err = fmt.Errorf("cliente Redis não configurado. Chame ConfigureClient primeiro")
			log.Printf("❌ %s", err)
			return
		}

		c := redis.NewClient(&redis.Options{
			Addr:         addr,
			MaxRetries:   2,
			MinIdleConns: 3,
			PoolSize:     10,
			PoolTimeout:  60 * time.Second,
		})

		pingErr := c.Ping(context.Background()).Err()
		if pingErr != nil {
			err = fmt.Errorf("falha ao conectar com o Redis em %s: %w", addr, pingErr)
			log.Printf("❌ %s", err.Error())
			return
		}

		log.Println("✅ Cliente Redis conectado e pronto para uso!")
		client = c
	})

	return client, err
}

func CloseRedisClient() {
	if client != nil {
		if err := client.Close(); err != nil {
			log.Printf("Erro ao fechar o cliente Redis: %v", err)
		}
	}
}
