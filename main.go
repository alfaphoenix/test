package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
)

// main запускает HTTP API и Telegram-бота.
func main() {
	config := LoadConfig()

	store, err := NewNotesStore(config.DatabaseURL)
	if err != nil {
		log.Fatalf("cannot init store: %v", err)
	}
	defer func() {
		if err := store.Close(); err != nil {
			log.Printf("cannot close store: %v", err)
		}
	}()

	api := NewAPI(store, config.APIUser, config.APIPassword)
	server := &http.Server{
		Addr:    config.HTTPAddr,
		Handler: api.Handler(),
	}

	bot := NewTelegramBot(store, config.BotToken, config.BotLogin, config.BotPassword)

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	go func() {
		log.Printf("HTTP server started on %s", config.HTTPAddr)
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
