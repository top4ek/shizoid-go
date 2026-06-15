package utils

import (
	"net/http"

	"go.uber.org/zap"

	"shizoid/internal/logger"
)

// Ping responds with JSON "OK" for health checks.
func Ping(w http.ResponseWriter, r *http.Request) {
	logger.Instance().Debug("health ping",
		zap.String("remote_addr", r.RemoteAddr),
		zap.String("method", r.Method),
	)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`"OK"`))
}

// HTTPWithPing serves /ping and delegates all other routes to fallback.
func HTTPWithPing(fallback http.Handler) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/ping", Ping)
	mux.Handle("/", fallback)
	return mux
}
