package main

import (
	"encoding/base64"
	"log"
	"net/http"
	"strings"
	"time"
)

// AuthMiddleware проверяет логин и пароль для HTTP API.
type AuthMiddleware struct {
	User     string
	Password string
}

// Wrap добавляет Basic Auth проверку к обработчику.
func (a AuthMiddleware) Wrap(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !a.authorized(r) {
			w.Header().Set("WWW-Authenticate", "Basic realm=notes")
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// authorized проверяет заголовок Authorization на соответствие логину и паролю.
func (a AuthMiddleware) authorized(r *http.Request) bool {
	if a.User == "" || a.Password == "" {
		return false
	}
	const prefix = "Basic "
	header := r.Header.Get("Authorization")
	if !strings.HasPrefix(header, prefix) {
		return false
	}
	payload, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(header, prefix))
	if err != nil {
		return false
	}
	parts := strings.SplitN(string(payload), ":", 2)
	if len(parts) != 2 {
		return false
	}
	return parts[0] == a.User && parts[1] == a.Password
}

// LoggingMiddleware выводит в лог информацию о запросе.
func LoggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		log.Printf("http %s %s %s", r.Method, r.URL.Path, time.Since(start))
	})
}
