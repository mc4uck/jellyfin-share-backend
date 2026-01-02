package middleware

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"net/http"
	"strings"
)

type contextKey string

const (
	ContextKeyClientIP contextKey = "clientIP"
	ContextKeyIPHash   contextKey = "ipHash"
)

// AdminAuth validates the backend API key for admin endpoints
func AdminAuth(apiKey string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			providedKey := r.Header.Get("X-Backend-Key")
			if providedKey == "" {
				providedKey = r.Header.Get("Authorization")
				providedKey = strings.TrimPrefix(providedKey, "Bearer ")
			}

			if subtle.ConstantTimeCompare([]byte(providedKey), []byte(apiKey)) != 1 {
				http.Error(w, `{"error": "unauthorized"}`, http.StatusUnauthorized)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// ClientIPMiddleware extracts and hashes the client IP
func ClientIPMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		clientIP := extractClientIP(r)
		ipHash := hashIP(clientIP)

		ctx := context.WithValue(r.Context(), ContextKeyClientIP, clientIP)
		ctx = context.WithValue(ctx, ContextKeyIPHash, ipHash)

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func extractClientIP(r *http.Request) string {
	// Check common headers for real IP behind proxies
	if cfIP := r.Header.Get("CF-Connecting-IP"); cfIP != "" {
		return cfIP
	}
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		parts := strings.Split(xff, ",")
		return strings.TrimSpace(parts[0])
	}
	if xrip := r.Header.Get("X-Real-IP"); xrip != "" {
		return xrip
	}

	// Fallback to remote address
	ip := r.RemoteAddr
	if colonIdx := strings.LastIndex(ip, ":"); colonIdx != -1 {
		ip = ip[:colonIdx]
	}
	return strings.Trim(ip, "[]")
}

func hashIP(ip string) string {
	hash := sha256.Sum256([]byte(ip))
	return hex.EncodeToString(hash[:8]) // First 8 bytes for privacy
}

func GetClientIP(ctx context.Context) string {
	if ip, ok := ctx.Value(ContextKeyClientIP).(string); ok {
		return ip
	}
	return ""
}

func GetIPHash(ctx context.Context) string {
	if hash, ok := ctx.Value(ContextKeyIPHash).(string); ok {
		return hash
	}
	return ""
}
