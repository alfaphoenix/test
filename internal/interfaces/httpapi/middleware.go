package httpapi

import (
	"encoding/base64"
	"log"
	"net/http"
	"strings"
	"time"
)

func BasicAuth(user, password string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !authorized(user, password, r) {
			w.Header().Set("WWW-Authenticate", "Basic realm=notes")
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func authorized(user, password string, r *http.Request) bool {
	if user == "" || password == "" {
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
	return len(parts) == 2 && parts[0] == user && parts[1] == password
}

func Logging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		log.Printf("http %s %s %s", r.Method, r.URL.Path, time.Since(start))
	})
}
