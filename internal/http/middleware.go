package httpapi

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/SolaTyolo/mcphub/internal/logx"
)

const httpTag = "http"

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, X-API-Key")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func accessLogMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/api/") {
			next.ServeHTTP(w, r)
			return
		}
		start := time.Now()
		rw := &statusWriter{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rw, r)
		logx.Info(httpTag, "%s %s status=%d duration=%s",
			r.Method, r.URL.Path, rw.status, time.Since(start).Round(time.Millisecond))
	})
}

type statusWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusWriter) WriteHeader(status int) {
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}

func (s *Server) withAPIKeyAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if s.cfg.GatewayAPIKey == "" {
			next(w, r)
			return
		}
		apiKey := strings.TrimSpace(r.Header.Get("X-API-Key"))
		if apiKey == "" || apiKey != s.cfg.GatewayAPIKey {
			logx.Warn(httpTag, "auth failed path=%s", r.URL.Path)
			writeError(w, http.StatusUnauthorized, errMissingAuth)
			return
		}
		ctx := context.WithValue(r.Context(), authContextKey, true)
		next(w, r.WithContext(ctx))
	}
}

type contextKey string

const authContextKey contextKey = "authed"

var errMissingAuth = &authError{msg: "X-API-Key header required"}

type authError struct{ msg string }

func (e *authError) Error() string { return e.msg }
