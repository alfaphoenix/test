package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"test/internal/config"
	"test/internal/infrastructure/postgres"
	"test/internal/interfaces/httpapi"
	"test/internal/interfaces/telegram"
	"test/internal/usecase"
)

func main() {
	cfg := config.Load()

	repo, err := postgres.NewStore(cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("cannot init store: %v", err)
	}
	defer func() {
		if err := repo.Close(); err != nil {
			log.Printf("cannot close store: %v", err)
		}
	}()

	service := usecase.NewNoteService(repo, cfg.BotLogin, cfg.BotPassword)
	handler := httpapi.NewHandler(service, cfg.APIUser, cfg.APIPassword)
	server := &http.Server{Addr: cfg.HTTPAddr, Handler: handler}
	bot := telegram.New(service, cfg.BotToken)

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	go func() {
		log.Printf("HTTP server started on %s", cfg.HTTPAddr)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("http server error: %v", err)
		}
	}()
	go func() {
		if err := bot.Start(ctx); err != nil {
			log.Fatalf("bot error: %v", err)
		}
	}()

	<-ctx.Done()
	log.Printf("shutdown requested")
	if err := server.Shutdown(context.Background()); err != nil {
		log.Printf("http shutdown error: %v", err)
	}
}
