package httpapi

import (
	"log"
	"net"
	"net/http"
	"strings"

	"codex-openai-wrapper/internal/config"
)

func withLogging(cfg config.Config, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip := clientIP(r)
		size := r.ContentLength
		log.Printf("request ip=%s method=%s path=%s size=%d", ip, r.Method, r.URL.Path, size)
		next.ServeHTTP(w, r)
	})
}

func clientIP(r *http.Request) string {
	if forwarded := r.Header.Get("X-Forwarded-For"); forwarded != "" {
		parts := strings.Split(forwarded, ",")
		return strings.TrimSpace(parts[0])
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
