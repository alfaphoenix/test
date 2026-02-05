package main

import "os"

// Config хранит настройки приложения из переменных окружения.
type Config struct {
	BotToken    string
	DatabaseURL string
	HTTPAddr    string
	APIUser     string
	APIPassword string
	BotLogin    string
	BotPassword string
}

// LoadConfig читает переменные окружения и возвращает заполненную конфигурацию.
func LoadConfig() Config {
	return Config{
		BotToken:    os.Getenv("BOT_TOKEN"),
		DatabaseURL: os.Getenv("DATABASE_URL"),
		HTTPAddr:    envOrDefault("HTTP_ADDR", ":8080"),
		APIUser:     os.Getenv("API_USER"),
		APIPassword: os.Getenv("API_PASSWORD"),
		BotLogin:    os.Getenv("BOT_LOGIN"),
		BotPassword: os.Getenv("BOT_PASSWORD"),
	}
}

// envOrDefault возвращает значение переменной окружения или значение по умолчанию.
func envOrDefault(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
