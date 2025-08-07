package libs

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

// GracefulShutdown inicia o servidor HTTP e gerencia seu desligamento gracioso.
// Ele escuta por sinais de interrupção (SIGINT, SIGTERM) e, quando recebidos,
// tenta desligar o servidor de forma segura em um tempo limite.
func GracefulShutdown(server *http.Server, timeout time.Duration) {
	// Inicia o servidor em uma goroutine para não bloquear o fluxo principal.
	go func() {
		log.Printf("🚀 Servidor HTTP escutando em: %s", server.Addr)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Falha ao iniciar o servidor: %v", err)
		}
	}()

	// Canal para receber sinais do sistema operacional.
	quit := make(chan os.Signal, 1)
	// Notifica o canal 'quit' quando os sinais SIGINT ou SIGTERM são recebidos.
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	// Bloqueia a execução até que um sinal seja recebido.
	<-quit

	log.Println("🔌 Desligando o servidor...")

	// Cria um contexto com tempo limite para o desligamento.
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	// Tenta desligar o servidor de forma graciosa.
	if err := server.Shutdown(ctx); err != nil {
		log.Fatalf("Erro no desligamento do servidor: %v", err)
	}

	log.Println("✅ Servidor desligado com sucesso.")
}
